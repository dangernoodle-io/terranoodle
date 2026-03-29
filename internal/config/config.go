package config

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"gopkg.in/yaml.v3"
)

const (
	ProjectFile = ".terranoodle.yml"
	GlobalFile  = "config.yml"
	GlobalDir   = "terranoodle"
)

// Config is the top-level terranoodle configuration.
type Config struct {
	Lint LintConfig `yaml:"lint,omitempty"`
}

// LintConfig holds lint-specific configuration.
type LintConfig struct {
	Rules       map[string]RuleConfig `yaml:"rules,omitempty"`
	ExcludeDirs []string              `yaml:"exclude-dirs,omitempty"`
	Overrides   []Override            `yaml:"overrides,omitempty"`
}

// Override applies different rule settings to paths matching glob patterns.
type Override struct {
	Paths []string              `yaml:"paths"`
	Rules map[string]RuleConfig `yaml:"rules"`
}

// RuleConfig supports both short form (bool) and long form (object with enabled + options).
// YAML: `rule: true` or `rule: {enabled: true, some-option: "value"}`.
type RuleConfig struct {
	Enabled  bool
	Severity string // "error" or "warn"; empty means "warn"
	Autofix  *bool  // nil means default (enabled), false means disabled, true means enabled
	Options  map[string]interface{}
}

// UnmarshalYAML handles both bool and object forms.
func (r *RuleConfig) UnmarshalYAML(node *yaml.Node) error {
	// Short form: `rule: true` or `rule: false`
	if node.Kind == yaml.ScalarNode {
		var enabled bool
		if err := node.Decode(&enabled); err != nil {
			return fmt.Errorf("rule config: expected bool or mapping, got %q", node.Value)
		}
		r.Enabled = enabled
		return nil
	}

	// Long form: `rule: {enabled: true, option: value}`
	if node.Kind == yaml.MappingNode {
		// Decode into a raw map first
		var raw map[string]interface{}
		if err := node.Decode(&raw); err != nil {
			return err
		}

		// Extract "enabled" key
		if v, ok := raw["enabled"]; ok {
			if b, ok := v.(bool); ok {
				r.Enabled = b
			}
			delete(raw, "enabled")
		}

		// Extract "severity" key
		if v, ok := raw["severity"]; ok {
			if s, ok := v.(string); ok {
				r.Severity = s
			}
			delete(raw, "severity")
		}

		// Extract "autofix" key
		if v, ok := raw["autofix"]; ok {
			if b, ok := v.(bool); ok {
				r.Autofix = &b
			}
			delete(raw, "autofix")
		}

		// Remaining keys are options
		if len(raw) > 0 {
			r.Options = raw
		}
		return nil
	}

	return fmt.Errorf("rule config: expected bool or mapping, got kind %d", node.Kind)
}

// MarshalYAML writes short form if no options and no severity, long form otherwise.
func (r RuleConfig) MarshalYAML() (interface{}, error) {
	if len(r.Options) == 0 && r.Severity == "" && r.Autofix == nil {
		return r.Enabled, nil
	}
	m := make(map[string]interface{}, len(r.Options)+3)
	m["enabled"] = r.Enabled
	if r.Severity != "" {
		m["severity"] = r.Severity
	}
	if r.Autofix != nil {
		m["autofix"] = *r.Autofix
	}
	for k, v := range r.Options {
		m[k] = v
	}
	return m, nil
}

// Load reads and parses a config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %q: %w", path, err)
	}

	return &cfg, nil
}

// Save writes a config to the given path as YAML.
func Save(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("config: create dir %q: %w", dir, err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("config: write %q: %w", path, err)
	}

	return nil
}

// GlobalPath returns the global config path: ~/.config/terranoodle/config.yml
// Uses a consistent path across all platforms.
func GlobalPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: user home dir: %w", err)
	}
	return filepath.Join(home, ".config", GlobalDir, GlobalFile), nil
}

// Discover finds and loads the effective config by:
// 1. Walking up from startDir to find .terranoodle.yml
// 2. Loading global config from ~/.config/terranoodle/config.yml (with profile support)
// 3. Merging: defaults <- global (with profile) <- project (project wins)
//
// Returns an empty Config (not nil) if no config files are found.
func Discover(startDir string) (*Config, error) {
	// Resolve to absolute path
	absDir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, fmt.Errorf("config: resolve %q: %w", startDir, err)
	}

	// Walk up to find project config
	var projectCfg *Config
	dir := absDir
	for {
		path := filepath.Join(dir, ProjectFile)
		if _, err := os.Stat(path); err == nil {
			projectCfg, err = Load(path)
			if err != nil {
				return nil, err
			}
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}

	// Load global config with profile support
	var effectiveGlobalLint LintConfig
	globalPath, err := GlobalPath()
	if err == nil {
		if _, err := os.Stat(globalPath); err == nil {
			globalCfg, err := LoadGlobal(globalPath)
			if err == nil {
				// Build effective global LintConfig with profile matching
				effectiveGlobalLint = buildEffectiveGlobalLint(globalCfg, absDir)
			}
			// Silently ignore errors loading global config
		}
	}

	// Merge: defaults <- global (with profile) <- project
	result := Default()
	if !isEmptyLintConfig(effectiveGlobalLint) {
		result = Merge(result, &Config{Lint: effectiveGlobalLint})
	}
	if projectCfg != nil {
		result = Merge(result, projectCfg)
	}
	return result, nil
}

// buildEffectiveGlobalLint builds the effective LintConfig from a GlobalConfig
// by merging: legacy flat lint <- default profile <- matched profile.
func buildEffectiveGlobalLint(cfg *GlobalConfig, cwd string) LintConfig {
	result := cfg.Lint // Start with legacy flat lint config

	// Merge default profile if it exists
	if defaultProfile, ok := cfg.Profiles["default"]; ok {
		tempCfg := &Config{Lint: result}
		profileCfg := &Config{Lint: defaultProfile.Lint}
		tempCfg = Merge(tempCfg, profileCfg)
		result = tempCfg.Lint
	}

	// Match and merge named profile if one matches the current working directory
	matchedName := MatchProfile(cfg, cwd)
	if matchedName != "" {
		if matchedProfile, ok := cfg.Profiles[matchedName]; ok {
			tempCfg := &Config{Lint: result}
			profileCfg := &Config{Lint: matchedProfile.Lint}
			tempCfg = Merge(tempCfg, profileCfg)
			result = tempCfg.Lint
		}
	}

	return result
}

// isEmptyLintConfig checks if a LintConfig has no meaningful content.
func isEmptyLintConfig(lc LintConfig) bool {
	return len(lc.Rules) == 0 && len(lc.ExcludeDirs) == 0 && len(lc.Overrides) == 0
}

// Merge returns a new Config with b's values overriding a's.
// Rules are merged per-key (b wins). ExcludeDirs from b replace a's.
// Overrides from b are appended after a's.
func Merge(a, b *Config) *Config {
	result := &Config{}

	// Merge rules
	result.Lint.Rules = make(map[string]RuleConfig)
	for k, v := range a.Lint.Rules {
		result.Lint.Rules[k] = v
	}
	for k, v := range b.Lint.Rules {
		result.Lint.Rules[k] = v
	}

	// ExcludeDirs: b replaces a if non-empty
	if len(b.Lint.ExcludeDirs) > 0 {
		result.Lint.ExcludeDirs = b.Lint.ExcludeDirs
	} else {
		result.Lint.ExcludeDirs = a.Lint.ExcludeDirs
	}

	// Overrides: append b after a (b's overrides apply later = higher priority)
	result.Lint.Overrides = append(a.Lint.Overrides, b.Lint.Overrides...)

	return result
}

// Default returns a Config with all built-in rules enabled.
func Default() *Config {
	rules := make(map[string]RuleConfig, len(Rules))
	for _, r := range Rules {
		rules[r.Name] = RuleConfig{Enabled: r.Default}
	}
	return &Config{
		Lint: LintConfig{
			Rules: rules,
		},
	}
}

//go:embed templates/config_long.yml.tmpl
var longTemplateFS embed.FS

var longTmpl = template.Must(
	template.New("config_long.yml.tmpl").
		ParseFS(longTemplateFS, "templates/config_long.yml.tmpl"),
)

type longTemplateData struct {
	Rules []RuleMeta
}

// RenderLong renders the annotated config template with all rule metadata.
func RenderLong(w io.Writer) error {
	return longTmpl.Execute(w, longTemplateData{Rules: Rules})
}
