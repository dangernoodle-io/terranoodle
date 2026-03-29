package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRulesCoverDefault(t *testing.T) {
	// Get all rule names from Rules
	rulesSet := make(map[string]bool)
	for _, r := range Rules {
		rulesSet[r.Name] = true
	}

	// Get all rule names from Default
	defaultCfg := Default()
	require.NotNil(t, defaultCfg)

	defaultRulesSet := make(map[string]bool)
	for ruleName := range defaultCfg.Lint.Rules {
		defaultRulesSet[ruleName] = true
	}

	// Assert they match
	assert.Equal(t, defaultRulesSet, rulesSet, "Rules slice should contain all rule names from Default()")
}
