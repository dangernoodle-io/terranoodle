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
		"allowed-filenames":        {Enabled: false},
		"versions-tf":              {Enabled: false},
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
		"allowed-filenames":        {Enabled: false},
		"versions-tf":              {Enabled: false},
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
		"allowed-filenames":        {Enabled: false},
		"versions-tf":              {Enabled: false},
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
		"allowed-filenames":        {Enabled: false},
		"versions-tf":              {Enabled: false},
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
		"unused-variable":   {Enabled: true},
		"allowed-filenames": {Enabled: false},
		"versions-tf":       {Enabled: false},
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
		"allowed-filenames":        {Enabled: false},
		"versions-tf":              {Enabled: false},
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

func TestModuleDir_AllowedFilenames(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"allowed-filenames": {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("allowed-filenames"), Options{Config: cfg})
	require.NoError(t, err)
	var filenameErrs []Error
	for _, e := range errs {
		if e.Kind == DisallowedFilename {
			filenameErrs = append(filenameErrs, e)
		}
	}
	require.Len(t, filenameErrs, 1)
	assert.Equal(t, "helpers.tf", filenameErrs[0].Variable)
}

func TestModuleDir_AllowedFilenames_Extended(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"allowed-filenames": {Enabled: true, Options: map[string]interface{}{
			"preset": "extended",
		}},
	}}
	errs, err := ModuleDir(moduleDirTestdata("allowed-filenames-extended"), Options{Config: cfg})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, DisallowedFilename, e.Kind)
	}
}

func TestModuleDir_AllowedFilenames_Additional(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"allowed-filenames": {Enabled: true, Options: map[string]interface{}{
			"additional": []interface{}{"helpers.tf"},
		}},
	}}
	errs, err := ModuleDir(moduleDirTestdata("allowed-filenames"), Options{Config: cfg})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, DisallowedFilename, e.Kind)
	}
}

func TestModuleDir_AllowedFilenames_DisabledByDefault(t *testing.T) {
	cfg := config.Default()
	errs, err := ModuleDir(moduleDirTestdata("allowed-filenames"), Options{Config: &cfg.Lint})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, DisallowedFilename, e.Kind)
	}
}

func TestModuleDir_VersionsTF_Missing(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"versions-tf": {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("versions-tf-missing"), Options{Config: cfg})
	require.NoError(t, err)
	var vErrs []Error
	for _, e := range errs {
		if e.Kind == MissingVersionsTF {
			vErrs = append(vErrs, e)
		}
	}
	require.Len(t, vErrs, 1)
}

func TestModuleDir_VersionsTF_NoTerraformBlock(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"versions-tf": {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("versions-tf-no-terraform"), Options{Config: cfg})
	require.NoError(t, err)
	var vErrs []Error
	for _, e := range errs {
		if e.Kind == MissingTerraformBlock {
			vErrs = append(vErrs, e)
		}
	}
	require.Len(t, vErrs, 1)
}

func TestModuleDir_VersionsTF_MissingSource(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"versions-tf": {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("versions-tf-invalid"), Options{Config: cfg})
	require.NoError(t, err)
	var sourceErrs []Error
	for _, e := range errs {
		if e.Kind == MissingProviderSource {
			sourceErrs = append(sourceErrs, e)
		}
	}
	require.NotEmpty(t, sourceErrs)
}

func TestModuleDir_VersionsTF_MissingVersion(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"versions-tf": {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("versions-tf-invalid"), Options{Config: cfg})
	require.NoError(t, err)
	var versionErrs []Error
	for _, e := range errs {
		if e.Kind == MissingProviderVersion {
			versionErrs = append(versionErrs, e)
		}
	}
	require.NotEmpty(t, versionErrs)
}

func TestModuleDir_VersionsTF_Duplicate(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"versions-tf": {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("versions-tf-duplicate"), Options{Config: cfg})
	require.NoError(t, err)
	var dupErrs []Error
	for _, e := range errs {
		if e.Kind == DuplicateProvider {
			dupErrs = append(dupErrs, e)
		}
	}
	require.Len(t, dupErrs, 1)
	assert.Equal(t, "aws", dupErrs[0].Variable)
}

func TestModuleDir_VersionsTF_Valid(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"versions-tf": {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("versions-tf-valid"), Options{Config: cfg})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, MissingVersionsTF, e.Kind)
		assert.NotEqual(t, MissingTerraformBlock, e.Kind)
		assert.NotEqual(t, MissingProviderSource, e.Kind)
		assert.NotEqual(t, MissingProviderVersion, e.Kind)
		assert.NotEqual(t, DuplicateProvider, e.Kind)
	}
}

func TestModuleDir_VersionsTF_DisabledByDefault(t *testing.T) {
	cfg := config.Default()
	errs, err := ModuleDir(moduleDirTestdata("versions-tf-missing"), Options{Config: &cfg.Lint})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, MissingVersionsTF, e.Kind)
	}
}

func TestModuleDir_SetStringType(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"set-string-type": {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("set-string-type"), Options{Config: cfg})
	require.NoError(t, err)
	var setErrs []Error
	for _, e := range errs {
		if e.Kind == SetStringType {
			setErrs = append(setErrs, e)
		}
	}
	require.Len(t, setErrs, 2)
}

func TestModuleDir_SetStringType_DisabledByDefault(t *testing.T) {
	cfg := config.Default()
	errs, err := ModuleDir(moduleDirTestdata("set-string-type"), Options{Config: &cfg.Lint})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, SetStringType, e.Kind)
	}
}

func TestModuleDir_ProviderConstraintStyle_Pessimistic(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"provider-constraint-style": {
				Enabled: true,
				Options: map[string]interface{}{
					"style": "pessimistic",
				},
			},
			"versions-tf": {Enabled: true},
		},
	}
	errs, err := ModuleDir(moduleDirTestdata("constraint-style-bad"), Options{Config: cfg})
	require.NoError(t, err)
	var matched []Error
	for _, e := range errs {
		if e.Kind == ProviderConstraintStyle {
			matched = append(matched, e)
		}
	}
	require.Len(t, matched, 1)
}

func TestModuleDir_ProviderConstraintStyle_PessimisticMajorDepth(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"provider-constraint-style": {
				Enabled: true,
				Options: map[string]interface{}{
					"style": "pessimistic",
					"depth": "major",
				},
			},
			"versions-tf": {Enabled: true},
		},
	}
	errs, err := ModuleDir(moduleDirTestdata("constraint-style-good"), Options{Config: cfg})
	require.NoError(t, err)
	var matched []Error
	for _, e := range errs {
		if e.Kind == ProviderConstraintStyle {
			matched = append(matched, e)
		}
	}
	require.Len(t, matched, 0)
}

func TestModuleDir_ProviderConstraintStyle_PessimisticMinorDepthFails(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"provider-constraint-style": {
				Enabled: true,
				Options: map[string]interface{}{
					"style": "pessimistic",
					"depth": "minor",
				},
			},
			"versions-tf": {Enabled: true},
		},
	}
	errs, err := ModuleDir(moduleDirTestdata("constraint-style-good"), Options{Config: cfg})
	require.NoError(t, err)
	var matched []Error
	for _, e := range errs {
		if e.Kind == ProviderConstraintStyle {
			matched = append(matched, e)
		}
	}
	require.Len(t, matched, 1)
}

func TestModuleDir_ProviderConstraintStyle_Exact(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"provider-constraint-style": {
				Enabled: true,
				Options: map[string]interface{}{
					"style": "exact",
				},
			},
			"versions-tf": {Enabled: true},
		},
	}
	errs, err := ModuleDir(moduleDirTestdata("constraint-style-exact"), Options{Config: cfg})
	require.NoError(t, err)
	var matched []Error
	for _, e := range errs {
		if e.Kind == ProviderConstraintStyle {
			matched = append(matched, e)
		}
	}
	require.Len(t, matched, 0)
}

func TestModuleDir_ProviderConstraintStyle_ExactFails(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"provider-constraint-style": {
				Enabled: true,
				Options: map[string]interface{}{
					"style": "exact",
				},
			},
			"versions-tf": {Enabled: true},
		},
	}
	errs, err := ModuleDir(moduleDirTestdata("constraint-style-good"), Options{Config: cfg})
	require.NoError(t, err)
	var matched []Error
	for _, e := range errs {
		if e.Kind == ProviderConstraintStyle {
			matched = append(matched, e)
		}
	}
	require.Len(t, matched, 1)
}

func TestModuleDir_ProviderConstraintStyle_DisabledByDefault(t *testing.T) {
	cfg := config.Default()
	errs, err := ModuleDir(moduleDirTestdata("constraint-style-bad"), Options{Config: &cfg.Lint})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, ProviderConstraintStyle, e.Kind)
	}
}
