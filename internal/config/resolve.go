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

// Get retrieves a value from the config by dot-path key.
// Supported keys: lint.rules.<name>, lint.exclude-dirs.
func (c *Config) Get(key string) (string, error) {
	parts := strings.SplitN(key, ".", 3)

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
		return strconv.FormatBool(rule.Enabled), nil

	case "exclude-dirs":
		return strings.Join(c.Lint.ExcludeDirs, ","), nil

	default:
		return "", fmt.Errorf("config: unknown key %q", key)
	}
}

// Set sets a value in the config by dot-path key.
// Supported keys: lint.rules.<name>, lint.exclude-dirs.
func (c *Config) Set(key, value string) error {
	parts := strings.SplitN(key, ".", 3)

	if len(parts) < 2 || parts[0] != "lint" {
		return fmt.Errorf("config: unknown key %q", key)
	}

	switch parts[1] {
	case "rules":
		if len(parts) < 3 {
			return fmt.Errorf("config: key %q requires a rule name", key)
		}
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("config: rule value must be true or false, got %q", value)
		}
		if c.Lint.Rules == nil {
			c.Lint.Rules = make(map[string]RuleConfig)
		}
		c.Lint.Rules[parts[2]] = RuleConfig{Enabled: enabled}
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
