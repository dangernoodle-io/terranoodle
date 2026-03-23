package validate

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// walkerTestdataDir returns the config directory for a named testdata scenario.
func walkerTestdataDir(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", name, "config")
}

// walkerTfTestdataDir returns the root directory for a TF-based testdata scenario.
func walkerTfTestdataDir(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", name, "root")
}

// walkerTestdataRoot returns the root directory of a testdata scenario.
func walkerTestdataRoot(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", name)
}

// TestDir_WithTerragruntValid tests Dir with valid terragrunt.hcl.
func TestDir_WithTerragruntValid(t *testing.T) {
	errs, err := Dir(walkerTestdataDir("simple-valid"))
	require.NoError(t, err)
	assert.Empty(t, errs)
}

// TestDir_WithTerragruntErrors tests Dir with terragrunt.hcl that has errors.
func TestDir_WithTerragruntErrors(t *testing.T) {
	errs, err := Dir(walkerTestdataDir("missing-required"))
	require.NoError(t, err)
	require.NotEmpty(t, errs, "expected validation errors")
	// Verify it's the right error type
	assert.Equal(t, MissingRequired, errs[0].Kind)
}

// TestDir_EmptyDir tests Dir with empty directory.
func TestDir_EmptyDir(t *testing.T) {
	tempDir := t.TempDir()
	errs, err := Dir(tempDir)
	require.NoError(t, err)
	assert.Empty(t, errs)
}

// TestDir_TFFilesFallback tests Dir falls back to TF validation when no terragrunt.hcl.
func TestDir_TFFilesFallback(t *testing.T) {
	errs, err := Dir(walkerTfTestdataDir("tf-simple-valid"))
	require.NoError(t, err)
	assert.Empty(t, errs)
}

// TestWalkDir_ValidScenario tests WalkDir with valid terragrunt config.
func TestWalkDir_ValidScenario(t *testing.T) {
	errs, err := WalkDir(walkerTestdataRoot("simple-valid"))
	require.NoError(t, err)
	assert.Empty(t, errs)
}

// TestWalkDir_SkipsHiddenDirs tests WalkDir skips hidden directories.
func TestWalkDir_SkipsHiddenDirs(t *testing.T) {
	tempDir := t.TempDir()

	// Create .hidden/terragrunt.hcl with invalid config
	hiddenDir := filepath.Join(tempDir, ".hidden")
	require.NoError(t, os.Mkdir(hiddenDir, 0o755))

	configPath := filepath.Join(hiddenDir, "terragrunt.hcl")
	invalidContent := `terraform {
  source = "invalid-source"
}
terraform_module {
  missing_required_var = true
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(invalidContent), 0o644))

	// WalkDir should skip .hidden and succeed
	errs, err := WalkDir(tempDir)
	require.NoError(t, err)
	assert.Empty(t, errs, "should skip .hidden directory")
}

// TestWalkDir_SkipsTerragruntCache tests WalkDir skips .terragrunt-cache.
func TestWalkDir_SkipsTerragruntCache(t *testing.T) {
	tempDir := t.TempDir()

	// Create .terragrunt-cache/terragrunt.hcl with invalid config
	cacheDir := filepath.Join(tempDir, ".terragrunt-cache")
	require.NoError(t, os.Mkdir(cacheDir, 0o755))

	configPath := filepath.Join(cacheDir, "terragrunt.hcl")
	invalidContent := `terraform {
  source = "invalid"
}
terraform_module {
  bad_config = true
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(invalidContent), 0o644))

	// WalkDir should skip .terragrunt-cache and succeed
	errs, err := WalkDir(tempDir)
	require.NoError(t, err)
	assert.Empty(t, errs, "should skip .terragrunt-cache directory")
}

// TestWalkDir_MultipleScenarios tests WalkDir encounters multiple configs.
func TestWalkDir_MultipleScenarios(t *testing.T) {
	// Create a temp directory with multiple subdirectories, each with terragrunt.hcl
	tempDir := t.TempDir()

	// Create a shared module directory
	moduleDir := filepath.Join(tempDir, "shared-module")
	require.NoError(t, os.Mkdir(moduleDir, 0o755))
	varsFile := filepath.Join(moduleDir, "variables.tf")
	require.NoError(t, os.WriteFile(varsFile, []byte(`variable "project_id" {
  type = string
}

variable "environment" {
  type = string
}
`), 0o644))

	// Create acme-project-1/terragrunt.hcl (valid)
	proj1 := filepath.Join(tempDir, "acme-project-1")
	require.NoError(t, os.Mkdir(proj1, 0o755))
	proj1Config := filepath.Join(proj1, "terragrunt.hcl")
	require.NoError(t, os.WriteFile(proj1Config, []byte(`terraform {
  source = "../shared-module"
}

inputs = {
  project_id  = "acme-project-1"
  environment = "dev"
}
`), 0o644))

	// Create acme-project-2/terragrunt.hcl (valid)
	proj2 := filepath.Join(tempDir, "acme-project-2")
	require.NoError(t, os.Mkdir(proj2, 0o755))
	proj2Config := filepath.Join(proj2, "terragrunt.hcl")
	require.NoError(t, os.WriteFile(proj2Config, []byte(`terraform {
  source = "../shared-module"
}

inputs = {
  project_id  = "acme-project-2"
  environment = "prod"
}
`), 0o644))

	errs, err := WalkDir(tempDir)
	require.NoError(t, err)
	assert.Empty(t, errs, "both configs should be valid")
}
