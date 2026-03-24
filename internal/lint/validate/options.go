package validate

import "dangernoodle.io/terranoodle/internal/config"

// Options configures lint validation behavior.
type Options struct {
	Config *config.LintConfig
}

// ruleNames maps ErrorKind to config rule names.
var ruleNames = map[ErrorKind]string{
	MissingRequired: "missing-required",
	ExtraInput:      "extra-input",
	TypeMismatch:    "type-mismatch",
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
