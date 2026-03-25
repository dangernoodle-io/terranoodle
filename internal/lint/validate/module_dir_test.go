package validate

import (
	"path/filepath"
	"runtime"
	"testing"

	"dangernoodle.io/terranoodle/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func moduleDirTestdata(name string) string { //nolint:unparam // name is generic for future testdata
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", name, "module")
}

func TestModuleDir_DisabledByDefault(t *testing.T) {
	cfg := config.Default()
	errs, err := ModuleDir(moduleDirTestdata("module-quality"), Options{Config: &cfg.Lint})
	require.NoError(t, err)
	assert.Empty(t, errs, "both rules disabled by default")
}

func TestModuleDir_MissingDescription(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"missing-description": {Enabled: true},
		"non-snake-case":      {Enabled: false},
	}}
	errs, err := ModuleDir(moduleDirTestdata("module-quality"), Options{Config: cfg})
	require.NoError(t, err)
	require.Len(t, errs, 2)
	for _, e := range errs {
		assert.Equal(t, MissingDescription, e.Kind)
	}
}

func TestModuleDir_NonSnakeCase(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"missing-description": {Enabled: false},
		"non-snake-case":      {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("module-quality"), Options{Config: cfg})
	require.NoError(t, err)
	require.Len(t, errs, 2)
	for _, e := range errs {
		assert.Equal(t, NonSnakeCase, e.Kind)
	}
}

func TestModuleDir_BothRulesEnabled(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"missing-description": {Enabled: true},
		"non-snake-case":      {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("module-quality"), Options{Config: cfg})
	require.NoError(t, err)
	assert.Len(t, errs, 4)
}

func TestModuleDir_AllClean(t *testing.T) {
	// Use an existing testdata module where all vars have descriptions and snake_case names
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "testdata", "simple-valid", "module")

	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"missing-description": {Enabled: true},
		"non-snake-case":      {Enabled: true},
	}}
	errs, err := ModuleDir(dir, Options{Config: cfg})
	require.NoError(t, err)
	assert.Empty(t, errs)
}

func TestModuleDir_EmptyDir(t *testing.T) {
	errs, err := ModuleDir(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, errs)
}

func TestModuleDir_NonexistentDir(t *testing.T) {
	_, err := ModuleDir("/nonexistent/acme-module")
	assert.Error(t, err)
}
