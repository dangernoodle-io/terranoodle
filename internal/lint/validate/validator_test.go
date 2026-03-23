package validate

import (
	"path/filepath"
	"runtime"
	"testing"

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
