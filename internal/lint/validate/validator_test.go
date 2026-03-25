package validate

import (
	"path/filepath"
	"runtime"
	"testing"

	"dangernoodle.io/terranoodle/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", name, "config", "terragrunt.hcl")
}

func TestFile_SimpleValid(t *testing.T) {
	errs, err := File(testdataPath("simple-valid"))
	require.NoError(t, err)
	assert.Empty(t, errs)
}

func TestFile_MissingRequired(t *testing.T) {
	errs, err := File(testdataPath("missing-required"))
	require.NoError(t, err)
	require.Len(t, errs, 2)

	names := map[string]bool{}
	for _, e := range errs {
		assert.Equal(t, MissingRequired, e.Kind)
		names[e.Variable] = true
	}
	assert.True(t, names["environment"], "expected missing: environment")
	assert.True(t, names["region"], "expected missing: region")
}

func TestFile_MissingRequiredWithTFVar(t *testing.T) {
	t.Setenv("TF_VAR_environment", "test")
	t.Setenv("TF_VAR_region", "us-east-1")

	errs, err := File(testdataPath("missing-required"))
	require.NoError(t, err)
	assert.Empty(t, errs, "all required variables should be satisfied by TF_VAR_* env vars")
}

func TestFile_MissingRequiredWithPartialTFVar(t *testing.T) {
	t.Setenv("TF_VAR_environment", "test")

	errs, err := File(testdataPath("missing-required"))
	require.NoError(t, err)
	require.Len(t, errs, 1)

	assert.Equal(t, MissingRequired, errs[0].Kind)
	assert.Equal(t, "region", errs[0].Variable, "expected only region to be missing")
}

func TestFile_ExtraInput(t *testing.T) {
	errs, err := File(testdataPath("extra-input"))
	require.NoError(t, err)
	require.Len(t, errs, 2)

	names := map[string]bool{}
	for _, e := range errs {
		assert.Equal(t, ExtraInput, e.Kind)
		names[e.Variable] = true
	}
	assert.True(t, names["environment"], "expected extra: environment")
	assert.True(t, names["bogus_field"], "expected extra: bogus_field")
}

func TestFile_MixedErrors(t *testing.T) {
	errs, err := File(testdataPath("mixed-errors"))
	require.NoError(t, err)
	require.Len(t, errs, 3)

	var missing, extra int
	for _, e := range errs {
		switch e.Kind {
		case MissingRequired:
			missing++
		case ExtraInput:
			extra++
		}
	}
	assert.Equal(t, 2, missing, "expected 2 missing required (environment, region)")
	assert.Equal(t, 1, extra, "expected 1 extra input (bogus_field)")
}

func TestFile_NoSource(t *testing.T) {
	errs, err := File(testdataPath("no-source"))
	require.NoError(t, err)
	assert.Empty(t, errs, "should skip validation when no source is present")
}

func TestFile_InterpolatedSource(t *testing.T) {
	errs, err := File(testdataPath("interpolated-source"))
	require.NoError(t, err)
	assert.Empty(t, errs, "interpolated source should resolve and validate clean")
}

func TestFile_GitCached(t *testing.T) {
	errs, err := File(testdataPath("git-cached"))
	require.NoError(t, err)
	require.Len(t, errs, 1)
	assert.Equal(t, MissingRequired, errs[0].Kind)
	assert.Equal(t, "region", errs[0].Variable)
}

func TestFile_GitCachedSubdir(t *testing.T) {
	errs, err := File(testdataPath("git-cached-subdir"))
	require.NoError(t, err)
	assert.Empty(t, errs, "git source with subdir should validate clean")
}

func TestFile_DepMergeExempt(t *testing.T) {
	errs, err := File(testdataPath("dep-merge-exempt"))
	require.NoError(t, err)
	assert.Empty(t, errs, "dep output keys not in child module vars should be exempt from extra-input check")
}

func TestFile_DepMergeExtra(t *testing.T) {
	errs, err := File(testdataPath("dep-merge-extra"))
	require.NoError(t, err)
	require.Len(t, errs, 1)
	assert.Equal(t, ExtraInput, errs[0].Kind)
	assert.Equal(t, "bogus_key", errs[0].Variable, "literal extra key should still be reported")
}

func TestFile_TypeMismatchSimple(t *testing.T) {
	errs, err := File(testdataPath("type-mismatch-simple"))
	require.NoError(t, err)
	require.Len(t, errs, 2)

	names := map[string]bool{}
	for _, e := range errs {
		assert.Equal(t, TypeMismatch, e.Kind)
		names[e.Variable] = true
	}
	assert.True(t, names["project_id"], "expected type mismatch: project_id")
	assert.True(t, names["enabled"], "expected type mismatch: enabled")
}

func TestFile_TypeMismatchObject(t *testing.T) {
	errs, err := File(testdataPath("type-mismatch-object"))
	require.NoError(t, err)
	require.Len(t, errs, 1)
	assert.Equal(t, TypeMismatch, errs[0].Kind)
	assert.Equal(t, "config", errs[0].Variable)
}

func TestFile_TypeValidCoercion(t *testing.T) {
	errs, err := File(testdataPath("type-valid-coercion"))
	require.NoError(t, err)
	assert.Empty(t, errs, "exact type match should produce no errors")
}

func TestFile_TypeAnySkip(t *testing.T) {
	errs, err := File(testdataPath("type-any-skip"))
	require.NoError(t, err)
	assert.Empty(t, errs, "type 'any' should accept everything")
}

func TestFile_IncludeExposeInputs(t *testing.T) {
	errs, err := File(testdataPath("include-expose-inputs"))
	require.NoError(t, err)
	assert.Empty(t, errs, "exposed include with inputs referenced in expressions should validate clean")
}

func TestFile_IncludeMergedInputs(t *testing.T) {
	errs, err := File(testdataPath("include-merged-inputs"))
	require.NoError(t, err)
	assert.Empty(t, errs, "merged inputs from non-exposed include should satisfy required variables")
}

func TestFile_TfVarsExtraInput(t *testing.T) {
	errs, err := File(testdataPath("tfvars-extra-input"))
	require.NoError(t, err)
	require.Len(t, errs, 1)

	assert.Equal(t, ExtraInput, errs[0].Kind)
	assert.Equal(t, "repository_screts", errs[0].Variable)
	assert.Contains(t, errs[0].Detail, "from tfvars file")
}

func TestFile_TfVarsProvidesRequired(t *testing.T) {
	errs, err := File(testdataPath("tfvars-provides-required"))
	require.NoError(t, err)
	assert.Empty(t, errs, "tfvars file providing required variable should satisfy requirement")
}

func tfTestdataDir(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", name, "root")
}

func TestTerraformDir_SimpleValid(t *testing.T) {
	errs, err := TerraformDir(tfTestdataDir("tf-simple-valid"))
	require.NoError(t, err)
	assert.Empty(t, errs)
}

func TestTerraformDir_MissingRequired(t *testing.T) {
	errs, err := TerraformDir(tfTestdataDir("tf-missing-required"))
	require.NoError(t, err)
	require.Len(t, errs, 2)

	names := map[string]bool{}
	for _, e := range errs {
		assert.Equal(t, MissingRequired, e.Kind)
		names[e.Variable] = true
		assert.Contains(t, e.Detail, `module "vpc"`)
	}
	assert.True(t, names["environment"], "expected missing: environment")
	assert.True(t, names["region"], "expected missing: region")
}

func TestTerraformDir_ExtraInput(t *testing.T) {
	errs, err := TerraformDir(tfTestdataDir("tf-extra-input"))
	require.NoError(t, err)
	require.Len(t, errs, 2)

	names := map[string]bool{}
	for _, e := range errs {
		assert.Equal(t, ExtraInput, e.Kind)
		names[e.Variable] = true
		assert.Contains(t, e.Detail, `module "vpc"`)
	}
	assert.True(t, names["environment"], "expected extra: environment")
	assert.True(t, names["bogus_field"], "expected extra: bogus_field")
}

func TestTerraformDir_NoModules(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "testdata", "tf-no-modules", "root")
	errs, err := TerraformDir(dir)
	require.NoError(t, err)
	assert.Empty(t, errs, "directory with no module blocks should produce no errors")
}

func TestTerraformDir_MultiModule(t *testing.T) {
	errs, err := TerraformDir(tfTestdataDir("tf-multi-module"))
	require.NoError(t, err)
	require.Len(t, errs, 2)

	var missing, extra int
	for _, e := range errs {
		assert.Contains(t, e.Detail, `module "bad"`)
		switch e.Kind {
		case MissingRequired:
			missing++
			assert.Equal(t, "region", e.Variable)
		case ExtraInput:
			extra++
			assert.Equal(t, "bogus", e.Variable)
		}
	}
	assert.Equal(t, 1, missing, "expected 1 missing required (region)")
	assert.Equal(t, 1, extra, "expected 1 extra input (bogus)")
}

func stackTestdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", name, "terragrunt.stack.hcl")
}

func TestStackFile_Valid(t *testing.T) {
	errs, err := StackFile(stackTestdataPath("stack-valid"))
	require.NoError(t, err)
	assert.Empty(t, errs)
}

func TestStackFile_Errors(t *testing.T) {
	errs, err := StackFile(stackTestdataPath("stack-errors"))
	require.NoError(t, err)
	require.Len(t, errs, 2)

	var missing, extra int
	for _, e := range errs {
		switch e.Kind {
		case MissingRequired:
			missing++
			assert.Contains(t, e.Detail, `unit "broken"`)
		case ExtraInput:
			extra++
			assert.Contains(t, e.Detail, `unit "broken"`)
		}
	}
	assert.Equal(t, 1, missing)
	assert.Equal(t, 1, extra)
}

func TestFile_RemoteSourceNoCache(t *testing.T) {
	_, err := File(testdataPath("remote-no-cache"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot resolve remote source")
}

func TestFile_ModulePathNotFound(t *testing.T) {
	_, err := File(testdataPath("module-path-missing"))
	assert.Error(t, err)
}

func TestStackFile_ParseError(t *testing.T) {
	_, err := StackFile("/nonexistent/path/terragrunt.stack.hcl")
	assert.Error(t, err)
}

func TestTerraformDir_ParseError(t *testing.T) {
	_, err := TerraformDir("/nonexistent/dir")
	assert.Error(t, err)
}

func TestFile_DepSkipBadDep(t *testing.T) {
	_, err := File(testdataPath("dep-bad-path"))
	// Bad dependencies are skipped, but we may still have validation issues
	// The dependency path is invalid, so it's skipped in resolveDepExemptions
	// The child-module is valid and resolvable
	// So err should be nil (no parse errors) and any errs should be about missing vars
	require.NoError(t, err)
	// The bad dependency is skipped gracefully, so we just get normal validation
	// The inputs merge(dependency.foo.outputs, {name: "test"}) becomes just {name: "test"}
	// which is insufficient for the required variables
}

func TestErrorKind_String(t *testing.T) {
	tests := []struct {
		kind     ErrorKind
		expected string
	}{
		{MissingRequired, "missing required input"},
		{ExtraInput, "extra input"},
		{TypeMismatch, "type mismatch"},
		{SourceRefSemver, "non-semver source ref"},
		{SourceProtocol, "disallowed source protocol"},
		{MissingDescription, "missing description"},
		{NonSnakeCase, "non-snake-case name"},
		{UnusedVariable, "UnusedVariable"},
		{OptionalWithoutDefault, "OptionalWithoutDefault"},
		{MissingIncludeExpose, "MissingIncludeExpose"},
		{DisallowedFilename, "disallowed filename"},
		{MissingVersionsTF, "missing versions.tf"},
		{MissingTerraformBlock, "missing terraform block"},
		{MissingProviderSource, "missing provider source"},
		{MissingProviderVersion, "missing provider version"},
		{DuplicateProvider, "duplicate provider"},
		{ErrorKind(999), "unknown"},
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			assert.Equal(t, test.expected, test.kind.String())
		})
	}
}

func TestFile_RuleFiltering(t *testing.T) {
	// Test without config - errors should be present
	errs, err := File(testdataPath("missing-required"))
	require.NoError(t, err)
	require.Len(t, errs, 2, "should have 2 missing required errors without config")

	// Test with config that disables missing-required rule
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"missing-required": {Enabled: false},
			"extra-input":      {Enabled: true},
			"type-mismatch":    {Enabled: true},
		},
	}
	opts := Options{Config: cfg}
	errs, err = File(testdataPath("missing-required"), opts)
	require.NoError(t, err)
	assert.Empty(t, errs, "errors should be filtered out when rule is disabled")
}

func TestFile_RuleFilteringMultipleKinds(t *testing.T) {
	// Test with mixed-errors fixture that has both missing-required and extra-input
	errs, err := File(testdataPath("mixed-errors"))
	require.NoError(t, err)
	require.Len(t, errs, 3, "should have 3 errors without config")

	// Test with config that disables missing-required only
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"missing-required": {Enabled: false},
			"extra-input":      {Enabled: true},
			"type-mismatch":    {Enabled: true},
		},
	}
	opts := Options{Config: cfg}
	errs, err = File(testdataPath("mixed-errors"), opts)
	require.NoError(t, err)
	require.Len(t, errs, 1, "should have only 1 extra-input error when missing-required disabled")
	assert.Equal(t, ExtraInput, errs[0].Kind)
}

func TestSeverityDefault(t *testing.T) {
	e := Error{}
	assert.Equal(t, SeverityError, e.Severity, "zero value should be SeverityError")
}

func TestErrorWithWarningSeverity(t *testing.T) {
	e := Error{Severity: SeverityWarning}
	assert.Equal(t, SeverityWarning, e.Severity)
}

func TestCheckSourceRef_Semver(t *testing.T) {
	sources := []string{
		"git::https://example.com/modules/vpc.git?ref=v1.0.0",
		"git::https://example.com/modules/vpc.git?ref=1.2.3",
		"git::https://example.com/modules/vpc.git?ref=v0.3.0-beta.1",
	}
	for _, src := range sources {
		errs := checkSourceRef(src, "/test/terragrunt.hcl", Options{})
		assert.Empty(t, errs, "expected no errors for source %q", src)
	}
}

func TestCheckSourceRef_NonSemver(t *testing.T) {
	sources := []string{
		"git::https://example.com/modules/vpc.git?ref=main",
		"git::https://example.com/modules/vpc.git?ref=master",
		"git::https://example.com/modules/vpc.git?ref=abc1234def5678",
	}
	for _, src := range sources {
		errs := checkSourceRef(src, "/test/terragrunt.hcl", Options{})
		require.Len(t, errs, 1, "expected 1 error for source %q", src)
		assert.Equal(t, SourceRefSemver, errs[0].Kind)
		assert.Equal(t, SeverityError, errs[0].Severity)
	}
}

func TestCheckSourceRef_AllowPatternMatch(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"source-ref-semver": {Enabled: true, Options: map[string]interface{}{
				"allow": []interface{}{"jae/*", "feature/*"},
			}},
		},
	}
	opts := Options{Config: cfg}
	errs := checkSourceRef("git::https://example.com/repo.git?ref=jae/add-widget", "/test/terragrunt.hcl", opts)
	require.Len(t, errs, 1)
	assert.Equal(t, SeverityWarning, errs[0].Severity)
}

func TestCheckSourceRef_AllowExactMatch(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"source-ref-semver": {Enabled: true, Options: map[string]interface{}{
				"allow": []interface{}{"develop"},
			}},
		},
	}
	opts := Options{Config: cfg}
	errs := checkSourceRef("git::https://example.com/repo.git?ref=develop", "/test/terragrunt.hcl", opts)
	require.Len(t, errs, 1)
	assert.Equal(t, SeverityWarning, errs[0].Severity)
}

func TestCheckSourceRef_AllowNoMatch(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"source-ref-semver": {Enabled: true, Options: map[string]interface{}{
				"allow": []interface{}{"jae/*"},
			}},
		},
	}
	opts := Options{Config: cfg}
	errs := checkSourceRef("git::https://example.com/repo.git?ref=main", "/test/terragrunt.hcl", opts)
	require.Len(t, errs, 1)
	assert.Equal(t, SeverityError, errs[0].Severity)
}

func TestCheckSourceRef_NoRef(t *testing.T) {
	errs := checkSourceRef("git::https://example.com/modules/vpc.git", "/test/terragrunt.hcl", Options{})
	assert.Empty(t, errs)
}

func TestCheckSourceRef_LocalSource(t *testing.T) {
	errs := checkSourceRef("../module", "/test/terragrunt.hcl", Options{})
	assert.Empty(t, errs)
}

func TestCheckSourceRef_TfrSource(t *testing.T) {
	errs := checkSourceRef("tfr://registry.example.com/modules/vpc?ref=main", "/test/terragrunt.hcl", Options{})
	assert.Empty(t, errs)
}

func TestFile_SourceRefSemver(t *testing.T) {
	errs, err := File(testdataPath("non-semver-ref"))
	require.NoError(t, err)
	require.Len(t, errs, 1)
	assert.Equal(t, SourceRefSemver, errs[0].Kind)
	assert.Equal(t, SeverityError, errs[0].Severity)
	assert.Contains(t, errs[0].Detail, "main")
}

func TestApplyAllowList(t *testing.T) {
	errs := []Error{
		{Variable: "environment", Kind: ExtraInput, Severity: SeverityError},
		{Variable: "bogus_field", Kind: ExtraInput, Severity: SeverityError},
		{Variable: "region", Kind: MissingRequired, Severity: SeverityError},
	}

	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"extra-input": {Enabled: true, Options: map[string]interface{}{
				"allow": []interface{}{"environment"},
			}},
		},
	}
	opts := Options{Config: cfg}
	result := applyAllowList(errs, opts)

	require.Len(t, result, 3)
	assert.Equal(t, SeverityWarning, result[0].Severity, "environment should be downgraded to warning")
	assert.Equal(t, SeverityError, result[1].Severity, "bogus_field should remain error")
	assert.Equal(t, SeverityError, result[2].Severity, "MissingRequired should not be affected")
}

func TestApplyAllowList_NoPatterns(t *testing.T) {
	errs := []Error{
		{Variable: "environment", Kind: ExtraInput, Severity: SeverityError},
	}
	result := applyAllowList(errs, Options{})
	require.Len(t, result, 1)
	assert.Equal(t, SeverityError, result[0].Severity, "should not change when no config")
}

func TestFile_ExtraInputAllow(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"extra-input": {Enabled: true, Options: map[string]interface{}{
				"allow": []interface{}{"environment"},
			}},
		},
	}
	opts := Options{Config: cfg}
	errs, err := File(testdataPath("extra-input"), opts)
	require.NoError(t, err)
	require.Len(t, errs, 2)

	for _, e := range errs {
		assert.Equal(t, ExtraInput, e.Kind)
		if e.Variable == "environment" {
			assert.Equal(t, SeverityWarning, e.Severity, "allowed variable should be warning")
		} else {
			assert.Equal(t, SeverityError, e.Severity, "non-allowed variable should be error")
			assert.Equal(t, "bogus_field", e.Variable)
		}
	}
}

func TestFile_ExtraInputAllowGlob(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"extra-input": {Enabled: true, Options: map[string]interface{}{
				"allow": []interface{}{"bogus_*"},
			}},
		},
	}
	opts := Options{Config: cfg}
	errs, err := File(testdataPath("extra-input"), opts)
	require.NoError(t, err)
	require.Len(t, errs, 2)

	for _, e := range errs {
		if e.Variable == "bogus_field" {
			assert.Equal(t, SeverityWarning, e.Severity, "glob-matched variable should be warning")
		} else {
			assert.Equal(t, SeverityError, e.Severity, "non-matched variable should be error")
		}
	}
}

func TestFile_ExtraInputAllowNoMatch(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"extra-input": {Enabled: true, Options: map[string]interface{}{
				"allow": []interface{}{"other"},
			}},
		},
	}
	opts := Options{Config: cfg}
	errs, err := File(testdataPath("extra-input"), opts)
	require.NoError(t, err)
	require.Len(t, errs, 2)

	for _, e := range errs {
		assert.Equal(t, SeverityError, e.Severity, "non-matching allow should keep error severity")
	}
}

func TestCheckSourceProtocol_LocalSource(t *testing.T) {
	errs := checkSourceProtocol("../module", "/test/terragrunt.hcl", Options{})
	assert.Empty(t, errs)
}

func TestCheckSourceProtocol_TfrSource(t *testing.T) {
	errs := checkSourceProtocol("tfr://registry.terraform.io/hashicorp/vpc/aws", "/test/terragrunt.hcl", Options{})
	assert.Empty(t, errs)
}

func TestCheckSourceProtocol_S3Source(t *testing.T) {
	errs := checkSourceProtocol("s3://acme-corp-modules/vpc", "/test/terragrunt.hcl", Options{})
	assert.Empty(t, errs)
}

func TestCheckSourceProtocol_NoEnforce(t *testing.T) {
	sshSrc := "git::git@github.com:acme-corp/modules.git//vpc?ref=v1.0.0"
	httpsSrc := "git::https://github.com/acme-corp/modules.git//vpc?ref=v1.0.0"
	assert.Empty(t, checkSourceProtocol(sshSrc, "/test/f.hcl", Options{}))
	assert.Empty(t, checkSourceProtocol(httpsSrc, "/test/f.hcl", Options{}))
}

func TestCheckSourceProtocol_EnforceHTTPS_FlagsSSH(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"source-protocol": {Enabled: true, Options: map[string]interface{}{
				"enforce": "https",
			}},
		},
	}
	opts := Options{Config: cfg}
	errs := checkSourceProtocol("git::git@github.com:acme-corp/modules.git//vpc?ref=v1.0.0", "/test/f.hcl", opts)
	require.Len(t, errs, 1)
	assert.Equal(t, SourceProtocol, errs[0].Kind)
	assert.Equal(t, SeverityError, errs[0].Severity)
	assert.Contains(t, errs[0].Detail, "SSH")
}

func TestCheckSourceProtocol_EnforceHTTPS_AllowsHTTPS(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"source-protocol": {Enabled: true, Options: map[string]interface{}{
				"enforce": "https",
			}},
		},
	}
	opts := Options{Config: cfg}
	sources := []string{
		"git::https://github.com/acme-corp/modules.git//vpc?ref=v1.0.0",
		"github.com/acme-corp/modules//vpc?ref=v1.0.0",
		"gitlab.com/acme-corp/modules//vpc?ref=v1.0.0",
	}
	for _, src := range sources {
		assert.Empty(t, checkSourceProtocol(src, "/test/f.hcl", opts), "expected no error for %q", src)
	}
}

func TestCheckSourceProtocol_EnforceSSH_FlagsHTTPS(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"source-protocol": {Enabled: true, Options: map[string]interface{}{
				"enforce": "ssh",
			}},
		},
	}
	opts := Options{Config: cfg}
	sources := []string{
		"git::https://github.com/acme-corp/modules.git//vpc?ref=v1.0.0",
		"github.com/acme-corp/modules//vpc?ref=v1.0.0",
		"gitlab.com/acme-corp/modules//vpc?ref=v1.0.0",
	}
	for _, src := range sources {
		errs := checkSourceProtocol(src, "/test/f.hcl", opts)
		require.Len(t, errs, 1, "expected 1 error for %q", src)
		assert.Equal(t, SourceProtocol, errs[0].Kind)
		assert.Contains(t, errs[0].Detail, "HTTPS")
	}
}

func TestCheckSourceProtocol_EnforceSSH_AllowsSSH(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"source-protocol": {Enabled: true, Options: map[string]interface{}{
				"enforce": "ssh",
			}},
		},
	}
	opts := Options{Config: cfg}
	errs := checkSourceProtocol("git::git@github.com:acme-corp/modules.git//vpc?ref=v1.0.0", "/test/f.hcl", opts)
	assert.Empty(t, errs)
}

func TestCheckSourceProtocol_EnforceAny(t *testing.T) {
	cfg := &config.LintConfig{
		Rules: map[string]config.RuleConfig{
			"source-protocol": {Enabled: true, Options: map[string]interface{}{
				"enforce": "any",
			}},
		},
	}
	opts := Options{Config: cfg}
	assert.Empty(t, checkSourceProtocol("git::git@github.com:acme-corp/modules.git//vpc", "/test/f.hcl", opts))
	assert.Empty(t, checkSourceProtocol("git::https://github.com/acme-corp/modules.git//vpc", "/test/f.hcl", opts))
}

func TestFile_MissingIncludeExpose(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"missing-include-expose": {Enabled: true},
	}}
	errs, err := File(testdataPath("missing-include-expose"), Options{Config: cfg})
	require.NoError(t, err)
	require.Len(t, errs, 1)
	assert.Equal(t, MissingIncludeExpose, errs[0].Kind)
	assert.Equal(t, "root", errs[0].Variable)
}

func TestFile_MissingIncludeExpose_HasExpose(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"missing-include-expose": {Enabled: true},
	}}
	errs, err := File(testdataPath("include-expose-inputs"), Options{Config: cfg})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, MissingIncludeExpose, e.Kind)
	}
}

func TestFile_MissingIncludeExpose_Excluded(t *testing.T) {
	cfg := &config.LintConfig{Rules: map[string]config.RuleConfig{
		"missing-include-expose": {Enabled: true, Options: map[string]interface{}{
			"exclude": []interface{}{"root"},
		}},
	}}
	errs, err := File(testdataPath("missing-include-expose"), Options{Config: cfg})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, MissingIncludeExpose, e.Kind)
	}
}

func TestFile_MissingIncludeExpose_DisabledByDefault(t *testing.T) {
	cfg := config.Default()
	errs, err := File(testdataPath("missing-include-expose"), Options{Config: &cfg.Lint})
	require.NoError(t, err)
	for _, e := range errs {
		assert.NotEqual(t, MissingIncludeExpose, e.Kind)
	}
}
