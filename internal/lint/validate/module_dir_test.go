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
		"unused-variables":         {Enabled: false},
		"optional-without-default": {Enabled: false},
		"allowed-filenames":        {Enabled: false},
		"has-versions-tf":          {Enabled: false},
		"missing-validation":       {Enabled: false},
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
		"unused-variables":         {Enabled: false},
		"optional-without-default": {Enabled: false},
		"allowed-filenames":        {Enabled: false},
		"has-versions-tf":          {Enabled: false},
		"missing-validation":       {Enabled: false},
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
		"unused-variables":         {Enabled: false},
		"optional-without-default": {Enabled: false},
		"allowed-filenames":        {Enabled: false},
		"has-versions-tf":          {Enabled: false},
		"missing-validation":       {Enabled: false},
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
		"unused-variables":         {Enabled: false},
		"optional-without-default": {Enabled: false},
		"allowed-filenames":        {Enabled: false},
		"has-versions-tf":          {Enabled: false},
		"missing-validation":       {Enabled: false},
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
		"unused-variables":   {Enabled: true},
		"allowed-filenames":  {Enabled: false},
		"has-versions-tf":    {Enabled: false},
		"missing-validation": {Enabled: false},
	}}
	errs, err := ModuleDir(moduleDirTestdata("unused-variables"), Options{Config: cfg})
	require.NoError(t, err)
	require.Len(t, errs, 1)
	assert.Equal(t, UnusedVariable, errs[0].Kind)
	assert.Equal(t, "unused_var", errs[0].Variable)
}

func TestModuleDir_UnusedVariable_DisabledByDefault(t *testing.T) {
	cfg := config.Default()
	errs, err := ModuleDir(moduleDirTestdata("unused-variables"), Options{Config: &cfg.Lint})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, UnusedVariable, e.Kind)
	}
}

func TestModuleDir_OptionalWithoutDefault(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"optional-without-default": {Enabled: true},
		"unused-variables":         {Enabled: false},
		"allowed-filenames":        {Enabled: false},
		"has-versions-tf":          {Enabled: false},
		"missing-validation":       {Enabled: false},
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
		"has-versions-tf": {Enabled: true},
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
		"has-versions-tf": {Enabled: true},
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
		"has-versions-tf": {Enabled: true},
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
		"has-versions-tf": {Enabled: true},
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
		"has-versions-tf": {Enabled: true},
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
		"has-versions-tf": {Enabled: true},
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
			"has-versions-tf": {Enabled: true},
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
			"has-versions-tf": {Enabled: true},
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
			"has-versions-tf": {Enabled: true},
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
			"has-versions-tf": {Enabled: true},
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
			"has-versions-tf": {Enabled: true},
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

func TestModuleDir_EmptyOutputsTF_DisabledByDefault(t *testing.T) {
	cfg := config.Default()
	errs, err := ModuleDir(moduleDirTestdata("empty-outputs-tf"), Options{Config: &cfg.Lint})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, EmptyOutputsTF, e.Kind)
	}
}

func TestModuleDir_EmptyOutputsTF_Enabled(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"empty-outputs-tf": {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("empty-outputs-tf"), Options{Config: cfg})
	require.NoError(t, err)
	var emptyErrs []Error
	for _, e := range errs {
		if e.Kind == EmptyOutputsTF {
			emptyErrs = append(emptyErrs, e)
		}
	}
	require.Len(t, emptyErrs, 1)
}

func TestModuleDir_EmptyOutputsTF_WithOutputs(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"empty-outputs-tf": {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("simple-valid"), Options{Config: cfg})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, EmptyOutputsTF, e.Kind)
	}
}

func TestModuleDir_VersionsTFSymlink_DisabledByDefault(t *testing.T) {
	cfg := config.Default()
	errs, err := ModuleDir(moduleDirTestdata("versions-tf-symlink"), Options{Config: &cfg.Lint})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, VersionsTFNotSymlink, e.Kind)
	}
}

func TestModuleDir_VersionsTFSymlink_NotSymlink(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"versions-tf-symlink": {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("versions-tf-symlink"), Options{Config: cfg})
	require.NoError(t, err)
	var symlinkErrs []Error
	for _, e := range errs {
		if e.Kind == VersionsTFNotSymlink {
			symlinkErrs = append(symlinkErrs, e)
		}
	}
	require.Len(t, symlinkErrs, 1)
	assert.Contains(t, symlinkErrs[0].Detail, "not a symlink")
}

func TestModuleDir_VersionsTFSymlink_Valid(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"versions-tf-symlink": {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("versions-tf-symlink-valid"), Options{Config: cfg})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, VersionsTFNotSymlink, e.Kind)
	}
}

func TestModuleDir_MissingValidation_DisabledByDefault(t *testing.T) {
	cfg := config.Default()
	errs, err := ModuleDir(moduleDirTestdata("missing-validation"), Options{Config: &cfg.Lint})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, MissingValidation, e.Kind)
	}
}

func TestModuleDir_MissingValidation_Enabled(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"missing-validation": {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("missing-validation"), Options{Config: cfg})
	require.NoError(t, err)
	var validationErrs []Error
	for _, e := range errs {
		if e.Kind == MissingValidation {
			validationErrs = append(validationErrs, e)
		}
	}
	require.Len(t, validationErrs, 1)
	assert.Equal(t, "project_id", validationErrs[0].Variable)
}

func TestModuleDir_MissingValidation_WithExclude(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"missing-validation": {
			Enabled: true,
			Options: map[string]interface{}{
				"exclude": []interface{}{"project_id"},
			},
		},
	}}
	errs, err := ModuleDir(moduleDirTestdata("missing-validation"), Options{Config: cfg})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, MissingValidation, e.Kind)
	}
}

func TestModuleDir_SensitiveOutput_DisabledByDefault(t *testing.T) {
	cfg := config.Default()
	errs, err := ModuleDir(moduleDirTestdata("sensitive-output"), Options{Config: &cfg.Lint})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, SensitiveOutput, e.Kind)
	}
}

func TestModuleDir_SensitiveOutput_Enabled(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"sensitive-output": {Enabled: true},
	}}
	errs, err := ModuleDir(moduleDirTestdata("sensitive-output"), Options{Config: cfg})
	require.NoError(t, err)
	var sensitiveErrs []Error
	for _, e := range errs {
		if e.Kind == SensitiveOutput {
			sensitiveErrs = append(sensitiveErrs, e)
		}
	}
	require.Len(t, sensitiveErrs, 1)
	assert.Equal(t, "api_key_value", sensitiveErrs[0].Variable)
}

func TestModuleDir_SensitiveOutput_AllSensitive(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "testdata", "simple-valid", "module")

	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"sensitive-output": {Enabled: true},
	}}
	errs, err := ModuleDir(dir, Options{Config: cfg})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, SensitiveOutput, e.Kind)
	}
}
