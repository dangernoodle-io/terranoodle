package validate

import (
	"testing"

	"dangernoodle.io/terranoodle/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestGetAllowPatterns(t *testing.T) {
	// nil config
	assert.Nil(t, getAllowPatterns(Options{}, "source-ref-semver"))

	// no rule
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{}}
	assert.Nil(t, getAllowPatterns(Options{Config: cfg}, "source-ref-semver"))

	// no allow option
	cfg = &config.LintConfig{Rules: map[string]config.RuleConfig{
		"source-ref-semver": {Enabled: true},
	}}
	assert.Nil(t, getAllowPatterns(Options{Config: cfg}, "source-ref-semver"))

	// with allow
	cfg = &config.LintConfig{Rules: map[string]config.RuleConfig{
		"source-ref-semver": {Enabled: true, Options: map[string]interface{}{
			"allow": []interface{}{"jae/*", "feature/*"},
		}},
	}}
	patterns := getAllowPatterns(Options{Config: cfg}, "source-ref-semver")
	assert.Equal(t, []string{"jae/*", "feature/*"}, patterns)
}
