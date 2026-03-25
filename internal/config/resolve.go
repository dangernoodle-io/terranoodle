package config

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// RulesForPath returns the effective rule-enabled map for a given file path.
// Starts with base rules, then applies matching overrides in order (last wins).
// relPath should be relative to the config file location.
func (c *LintConfig) RulesForPath(relPath string) map[string]bool {
	result := make(map[string]bool)

	// Base rules
	for name, rule := range c.Rules {
		result[name] = rule.Enabled
	}

	// Apply matching overrides in order
	for _, override := range c.Overrides {
		if pathMatchesAny(relPath, override.Paths) {
			for name, rule := range override.Rules {
				result[name] = rule.Enabled
			}
		}
	}

	return result
}

// IsRuleEnabled checks if a specific rule is enabled for the given path.
// Returns true if the rule is not configured (unknown rules default to enabled).
func (c *LintConfig) IsRuleEnabled(ruleName, relPath string) bool {
	rules := c.RulesForPath(relPath)
	enabled, ok := rules[ruleName]
	if !ok {
		return true // unknown rules default to enabled
	}
	return enabled
}

// RuleSeverity returns the configured severity for a rule at a given path.
// Returns "error" if severity is explicitly set to "error", otherwise "warn" (default).
func (c *LintConfig) RuleSeverity(ruleName, filePath string) string {
	rule, ok := c.Rules[ruleName]
	if !ok {
		return "warn"
	}
	if rule.Severity == "error" {
		return "error"
	}
	return "warn"
}

// pathMatchesAny checks if path matches any of the glob patterns.
func pathMatchesAny(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		// Also try matching against just the filename for simple patterns
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}

// parseOptionValue parses a CLI string value into the appropriate type.
// Comma-separated values become []interface{} lists.
// Otherwise tries bool, int, float, falls back to string.
func parseOptionValue(value string) interface{} {
	if strings.Contains(value, ",") {
		parts := strings.Split(value, ",")
		result := make([]interface{}, len(parts))
		for i, p := range parts {
			result[i] = strings.TrimSpace(p)
		}
		return result
	}
	if b, err := strconv.ParseBool(value); err == nil {
		return b
	}
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		return int(n)
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}
	return value
}

// Get retrieves a value from the config by dot-path key.
// Supported keys: lint.rules.<name>, lint.rules.<name>.<option>, lint.exclude-dirs.
func (c *Config) Get(key string) (string, error) {
	parts := strings.SplitN(key, ".", 4)

	if len(parts) < 2 || parts[0] != "lint" {
		return "", fmt.Errorf("config: unknown key %q", key)
	}

	switch parts[1] {
	case "rules":
		if len(parts) < 3 {
			return "", fmt.Errorf("config: key %q requires a rule name", key)
		}
		rule, ok := c.Lint.Rules[parts[2]]
		if !ok {
			return "", fmt.Errorf("config: rule %q not configured", parts[2])
		}

		// Get rule option if requested
		if len(parts) == 4 {
			ruleName := parts[2]
			optionName := parts[3]
			val, ok := rule.Options[optionName]
			if !ok {
				return "", fmt.Errorf("config: rule %q has no option %q", ruleName, optionName)
			}
			// Stringify the value
			switch v := val.(type) {
			case []interface{}:
				strs := make([]string, len(v))
				for i, item := range v {
					strs[i] = fmt.Sprintf("%v", item)
				}
				return strings.Join(strs, ","), nil
			default:
				return fmt.Sprintf("%v", val), nil
			}
		}

		return strconv.FormatBool(rule.Enabled), nil

	case "exclude-dirs":
		return strings.Join(c.Lint.ExcludeDirs, ","), nil

	default:
		return "", fmt.Errorf("config: unknown key %q", key)
	}
}

// Set sets a value in the config by dot-path key.
// Supported keys: lint.rules.<name>, lint.rules.<name>.<option>, lint.exclude-dirs.
func (c *Config) Set(key, value string) error {
	parts := strings.SplitN(key, ".", 4)

	if len(parts) < 2 || parts[0] != "lint" {
		return fmt.Errorf("config: unknown key %q", key)
	}

	switch parts[1] {
	case "rules":
		if len(parts) < 3 {
			return fmt.Errorf("config: key %q requires a rule name", key)
		}

		if c.Lint.Rules == nil {
			c.Lint.Rules = make(map[string]RuleConfig)
		}

		// Set rule option if 4-part key
		if len(parts) == 4 {
			ruleName := parts[2]
			optionName := parts[3]
			existing := c.Lint.Rules[ruleName]
			if existing.Options == nil {
				existing.Options = make(map[string]interface{})
			}
			existing.Options[optionName] = parseOptionValue(value)
			c.Lint.Rules[ruleName] = existing
			return nil
		}

		// Set rule enabled flag (3-part key)
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("config: rule value must be true or false, got %q", value)
		}
		existing := c.Lint.Rules[parts[2]]
		existing.Enabled = enabled
		c.Lint.Rules[parts[2]] = existing
		return nil

	case "exclude-dirs":
		if value == "" {
			c.Lint.ExcludeDirs = nil
		} else {
			c.Lint.ExcludeDirs = strings.Split(value, ",")
		}
		return nil

	default:
		return fmt.Errorf("config: unknown key %q", key)
	}
}
