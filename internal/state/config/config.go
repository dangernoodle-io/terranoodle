package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level mapping configuration.
type Config struct {
	API       *API                   `yaml:"api,omitempty"`
	Vars      map[string]string      `yaml:"vars"`
	Resolvers map[string]Resolver    `yaml:"resolvers"`
	Types     map[string]TypeMapping `yaml:"types"`
}

// API holds the optional REST API connection settings used by resolvers.
type API struct {
	BaseURL     string `yaml:"base_url,omitempty"`
	TokenEnv    string `yaml:"token_env"`
	OpenAPISpec string `yaml:"openapi_spec,omitempty"`
}

// Resolver describes how to fetch a value from the API.
type Resolver struct {
	Use  []string    `yaml:"use,omitempty"`
	Get  string      `yaml:"get"`
	Pick interface{} `yaml:"pick"` // string or PickExpr
}

// PickExpr is the structured form of a pick expression with a where filter.
type PickExpr struct {
	Where map[string]string `yaml:"where"`
	Field string            `yaml:"field"`
}

// TypeMapping maps a Terraform resource type to its import ID template.
type TypeMapping struct {
	Use []string `yaml:"use,omitempty"`
	ID  string   `yaml:"id"`
}

// Load reads the YAML file at path and unmarshals it into a Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal %q: %w", path, err)
	}

	return &cfg, nil
}

// ParsePick determines whether raw is a simple field name string, a jq
// expression string, or a structured PickExpr (with where/field keys).
//
// Returns:
//   - (field, nil, "", nil)    when raw is a plain string (no jq indicators).
//   - ("", nil, jqExpr, nil)   when raw is a jq expression string (starts with
//     "." and contains "|", "[]", or "select(").
//   - ("", expr, "", nil)      when raw is a where/field map.
//   - ("", nil, "", err)       when raw is neither.
func ParsePick(raw interface{}) (field string, expr *PickExpr, jqExpr string, err error) {
	if raw == nil {
		return "", nil, "", fmt.Errorf("config: pick is nil")
	}

	switch v := raw.(type) {
	case string:
		if isJQExpression(v) {
			return "", nil, v, nil
		}
		return v, nil, "", nil

	case map[string]interface{}:
		// Re-encode and decode via yaml to reuse the PickExpr struct tags.
		encoded, err := yaml.Marshal(v)
		if err != nil {
			return "", nil, "", fmt.Errorf("config: pick re-encode: %w", err)
		}
		var pickExpr PickExpr
		if err := yaml.Unmarshal(encoded, &pickExpr); err != nil {
			return "", nil, "", fmt.Errorf("config: pick decode: %w", err)
		}
		return "", &pickExpr, "", nil

	default:
		return "", nil, "", fmt.Errorf("config: pick must be a string or where/field map, got %T", raw)
	}
}

// isJQExpression reports whether s is a jq-style expression rather than a
// plain field name. A string is treated as a jq expression when it starts
// with "." and contains at least one of: "|", "[]", "[<digit>", or "select(".
func isJQExpression(s string) bool {
	if len(s) == 0 || s[0] != '.' {
		return false
	}
	if strings.Contains(s, "|") ||
		strings.Contains(s, "[]") ||
		strings.Contains(s, "select(") {
		return true
	}
	// Detect array-index access like .[0] or .items[1].
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '[' && s[i+1] >= '0' && s[i+1] <= '9' {
			return true
		}
	}
	return false
}
