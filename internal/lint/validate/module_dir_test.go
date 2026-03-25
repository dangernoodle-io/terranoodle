package validate

import (
	"path/filepath"
	"runtime"
	"testing"

	"dangernoodle.io/terranoodle/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func moduleDirTestdata(name string) string {
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
		"missing-description":      {Enabled: true},
		"non-snake-case":           {Enabled: false},
		"unused-variable":          {Enabled: false},
		"optional-without-default": {Enabled: false},
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
		"missing-description":      {Enabled: false},
		"non-snake-case":           {Enabled: true},
		"unused-variable":          {Enabled: false},
		"optional-without-default": {Enabled: false},
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
		"missing-description":      {Enabled: true},
		"non-snake-case":           {Enabled: true},
		"unused-variable":          {Enabled: false},
		"optional-without-default": {Enabled: false},
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
		"missing-description":      {Enabled: true},
		"non-snake-case":           {Enabled: true},
		"unused-variable":          {Enabled: false},
		"optional-without-default": {Enabled: false},
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

func TestModuleDir_UnusedVariable(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"unused-variable": {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("unused-variable"), Options{Config: cfg})
	require.NoError(t, err)
	require.Len(t, errs, 1)
	assert.Equal(t, UnusedVariable, errs[0].Kind)
	assert.Equal(t, "unused_var", errs[0].Variable)
}

func TestModuleDir_UnusedVariable_DisabledByDefault(t *testing.T) {
	cfg := config.Default()
	errs, err := ModuleDir(moduleDirTestdata("unused-variable"), Options{Config: &cfg.Lint})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, UnusedVariable, e.Kind)
	}
}

func TestModuleDir_OptionalWithoutDefault(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"optional-without-default": {Enabled: true},
		"unused-variable":          {Enabled: false},
	}}
	errs, err := ModuleDir(moduleDirTestdata("optional-no-default"), Options{Config: cfg})
	require.NoError(t, err)
	require.Len(t, errs, 2)
	for _, e := range errs {
		assert.Equal(t, OptionalWithoutDefault, e.Kind)
	}
}

func TestModuleDir_OptionalWithoutDefault_DisabledByDefault(t *testing.T) {
	cfg := config.Default()
	errs, err := ModuleDir(moduleDirTestdata("optional-no-default"), Options{Config: &cfg.Lint})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, OptionalWithoutDefault, e.Kind)
	}
}
