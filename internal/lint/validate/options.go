package validate

import "dangernoodle.io/terranoodle/internal/config"

// Options configures lint validation behavior.
type Options struct {
	Config *config.LintConfig
	Strict bool
}

// ruleNames maps ErrorKind to config rule names.
var ruleNames = map[ErrorKind]string{
	MissingRequired:        "missing-required",
	ExtraInput:             "extra-input",
	TypeMismatch:           "type-mismatch",
	SourceRefSemver:        "source-ref-semver",
	SourceProtocol:         "source-protocol",
	MissingDescription:     "missing-description",
	NonSnakeCase:           "non-snake-case",
	UnusedVariable:         "unused-variable",
	OptionalWithoutDefault: "optional-without-default",
}

// filterErrors removes errors for disabled rules.
func filterErrors(errs []Error, opts Options) []Error {
	if opts.Config == nil {
		return errs
	}

	filtered := make([]Error, 0, len(errs))
	for _, e := range errs {
		ruleName, ok := ruleNames[e.Kind]
		if !ok || opts.Config.IsRuleEnabled(ruleName, e.File) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// isExcludedDir checks if a directory name matches ExcludeDirs.
func isExcludedDir(name string, opts Options) bool {
	if opts.Config == nil {
		return false
	}
	for _, excl := range opts.Config.ExcludeDirs {
		if name == excl || name+"/" == excl {
			return true
		}
	}
	return false
}

// getAllowPatterns reads the "allow" option from a rule's config.
func getAllowPatterns(opts Options, ruleName string) []string { //nolint:unparam // ruleName is generic for future rules
	if opts.Config == nil {
		return nil
	}
	rule, ok := opts.Config.Rules[ruleName]
	if !ok {
		return nil
	}
	raw, ok := rule.Options["allow"]
	if !ok {
		return nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	patterns := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			patterns = append(patterns, s)
		}
	}
	return patterns
}

// getEnforceOption reads the "enforce" option from a rule's config.
func getEnforceOption(opts Options, ruleName string) string { //nolint:unparam // ruleName is generic for future rules
	if opts.Config == nil {
		return ""
	}
	rule, ok := opts.Config.Rules[ruleName]
	if !ok {
		return ""
	}
	raw, ok := rule.Options["enforce"]
	if !ok {
		return ""
	}
	if s, ok := raw.(string); ok {
		return s
	}
	return ""
}
