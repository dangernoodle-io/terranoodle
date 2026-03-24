package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dangernoodle.io/terranoodle/internal/state/rename"
)

// parseVarFlags tests

func TestParseVarFlags_Valid(t *testing.T) {
	vars := []string{"key=value", "foo=bar"}
	result, err := parseVarFlags(vars)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"key": "value", "foo": "bar"}, result)
}

func TestParseVarFlags_ValueWithEquals(t *testing.T) {
	vars := []string{"key=val=ue"}
	result, err := parseVarFlags(vars)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"key": "val=ue"}, result)
}

func TestParseVarFlags_EmptyValue(t *testing.T) {
	vars := []string{"key="}
	result, err := parseVarFlags(vars)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"key": ""}, result)
}

func TestParseVarFlags_MissingEquals(t *testing.T) {
	vars := []string{"invalid"}
	result, err := parseVarFlags(vars)
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected key=value")
}

func TestParseVarFlags_EmptyInput(t *testing.T) {
	result, err := parseVarFlags(nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{}, result)
}

// filterManaged tests

func TestFilterManaged_NoManaged(t *testing.T) {
	creates := []*tfjson.ResourceChange{
		{Address: "aws_s3_bucket.acme", Type: "aws_s3_bucket"},
		{Address: "aws_iam_role.app", Type: "aws_iam_role"},
	}
	result := filterManaged(creates, nil)
	assert.Equal(t, 2, len(result))
	assert.Equal(t, creates, result)
}

func TestFilterManaged_SomeManaged(t *testing.T) {
	creates := []*tfjson.ResourceChange{
		{Address: "aws_s3_bucket.acme", Type: "aws_s3_bucket"},
		{Address: "aws_iam_role.app", Type: "aws_iam_role"},
		{Address: "aws_iam_policy.policy", Type: "aws_iam_policy"},
	}
	managed := []string{"aws_iam_role.app"}
	result := filterManaged(creates, managed)
	assert.Equal(t, 2, len(result))
	assert.Equal(t, "aws_s3_bucket.acme", result[0].Address)
	assert.Equal(t, "aws_iam_policy.policy", result[1].Address)
}

func TestFilterManaged_AllManaged(t *testing.T) {
	creates := []*tfjson.ResourceChange{
		{Address: "aws_s3_bucket.acme", Type: "aws_s3_bucket"},
		{Address: "aws_iam_role.app", Type: "aws_iam_role"},
	}
	managed := []string{"aws_s3_bucket.acme", "aws_iam_role.app"}
	result := filterManaged(creates, managed)
	assert.Equal(t, 0, len(result))
}

// extractFields tests

func TestExtractFields_StringValues(t *testing.T) {
	after := map[string]interface{}{
		"name":   "acme-bucket",
		"region": "us-east-1",
	}
	result := extractFields(after)
	assert.Equal(t, map[string]string{
		"name":   "acme-bucket",
		"region": "us-east-1",
	}, result)
}

func TestExtractFields_MixedTypes(t *testing.T) {
	after := map[string]interface{}{
		"name":    "acme",
		"enabled": true,
		"count":   float64(3),
	}
	result := extractFields(after)
	assert.Equal(t, map[string]string{
		"name":    "acme",
		"enabled": "true",
		"count":   "3",
	}, result)
}

func TestExtractFields_NilAfter(t *testing.T) {
	result := extractFields(nil)
	assert.Equal(t, map[string]string{}, result)
}

func TestExtractFields_NonMapType(t *testing.T) {
	result := extractFields("not a map")
	assert.Equal(t, map[string]string{}, result)
}

func TestExtractFields_NestedTypesSkipped(t *testing.T) {
	after := map[string]interface{}{
		"name": "acme",
		"tags": map[string]interface{}{
			"env": "staging",
		},
	}
	result := extractFields(after)
	// nested map should be skipped
	assert.Equal(t, map[string]string{"name": "acme"}, result)
	assert.NotContains(t, result, "tags")
}

// resolveDir tests

func TestResolveDir_NonEmptyFlag(t *testing.T) {
	dir, err := resolveDir("/some/path")
	require.NoError(t, err)
	assert.Equal(t, "/some/path", dir)
}

func TestResolveDir_EmptyFlag(t *testing.T) {
	dir, err := resolveDir("")
	require.NoError(t, err)
	cwd, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, cwd, dir)
}

// detectTerragrunt tests

func TestDetectTerragrunt_WithCache(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := tmpDir + "/.terragrunt-cache"
	err := os.Mkdir(cacheDir, 0755)
	require.NoError(t, err)

	result := detectTerragrunt(tmpDir)
	assert.True(t, result)
}

func TestDetectTerragrunt_WithoutCache(t *testing.T) {
	tmpDir := t.TempDir()
	result := detectTerragrunt(tmpDir)
	assert.False(t, result)
}

// Root command tests

func TestRootCmd_Version(t *testing.T) {
	oldVersion := Version
	oldFlag := versionFlag
	t.Cleanup(func() {
		Version = oldVersion
		versionFlag = oldFlag
	})

	Version = "v0.1.0-test"
	versionFlag = true

	err := rootCmd.RunE(rootCmd, nil)
	require.NoError(t, err)
}

func TestRootCmd_Help(t *testing.T) {
	oldFlag := versionFlag
	t.Cleanup(func() { versionFlag = oldFlag })

	versionFlag = false
	err := rootCmd.RunE(rootCmd, nil)
	require.NoError(t, err)
}

func TestRootCmd_NoColor(t *testing.T) {
	err := rootCmd.Flags().Set("no-color", "true")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = rootCmd.Flags().Set("no-color", "false")
	})

	err = rootCmd.PersistentPreRunE(rootCmd, nil)
	require.NoError(t, err)
}

// Lint command tests

func TestRunLint_ValidDir(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := tmpDir + "/config"
	moduleDir := tmpDir + "/module"

	err := os.Mkdir(configDir, 0755)
	require.NoError(t, err)
	err = os.Mkdir(moduleDir, 0755)
	require.NoError(t, err)

	// Create terragrunt.hcl
	terragruntContent := `terraform {
  source = "../module"
}

inputs = {
  project_id = "prj-test-001"
  environment = "dev"
}`
	err = os.WriteFile(configDir+"/terragrunt.hcl", []byte(terragruntContent), 0644)
	require.NoError(t, err)

	// Create variables.tf
	variablesContent := `variable "project_id" {
  type = string
}

variable "environment" {
  type = string
}

variable "labels" {
  default = {}
  type = map(string)
}`
	err = os.WriteFile(moduleDir+"/variables.tf", []byte(variablesContent), 0644)
	require.NoError(t, err)

	oldDirFlag := lintDirFlag
	oldAllFlag := lintAllFlag
	t.Cleanup(func() {
		lintDirFlag = oldDirFlag
		lintAllFlag = oldAllFlag
	})

	lintDirFlag = configDir
	lintAllFlag = false

	err = runLint(lintCmd, nil)
	require.NoError(t, err)
}

func TestRunLint_InvalidDir(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := tmpDir + "/config"
	moduleDir := tmpDir + "/module"

	err := os.Mkdir(configDir, 0755)
	require.NoError(t, err)
	err = os.Mkdir(moduleDir, 0755)
	require.NoError(t, err)

	// Create terragrunt.hcl with missing "environment" input
	terragruntContent := `terraform {
  source = "../module"
}

inputs = {
  project_id = "prj-test-001"
}`
	err = os.WriteFile(configDir+"/terragrunt.hcl", []byte(terragruntContent), 0644)
	require.NoError(t, err)

	// Create variables.tf with "environment" required
	variablesContent := `variable "project_id" {
  type = string
}

variable "environment" {
  type = string
}

variable "region" {
  type = string
}

variable "labels" {
  default = {}
  type = map(string)
}`
	err = os.WriteFile(moduleDir+"/variables.tf", []byte(variablesContent), 0644)
	require.NoError(t, err)

	oldDirFlag := lintDirFlag
	oldAllFlag := lintAllFlag
	t.Cleanup(func() {
		lintDirFlag = oldDirFlag
		lintAllFlag = oldAllFlag
	})

	lintDirFlag = configDir
	lintAllFlag = false

	err = runLint(lintCmd, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "issue")
}

func TestRunLint_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	oldDirFlag := lintDirFlag
	oldAllFlag := lintAllFlag
	t.Cleanup(func() {
		lintDirFlag = oldDirFlag
		lintAllFlag = oldAllFlag
	})

	lintDirFlag = tmpDir
	lintAllFlag = false

	err := runLint(lintCmd, nil)
	require.NoError(t, err)
}

func TestRunLint_All(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := tmpDir + "/config"
	moduleDir := tmpDir + "/module"

	err := os.Mkdir(configDir, 0755)
	require.NoError(t, err)
	err = os.Mkdir(moduleDir, 0755)
	require.NoError(t, err)

	// Create terragrunt.hcl
	terragruntContent := `terraform {
  source = "../module"
}

inputs = {
  project_id = "prj-test-001"
  environment = "dev"
}`
	err = os.WriteFile(configDir+"/terragrunt.hcl", []byte(terragruntContent), 0644)
	require.NoError(t, err)

	// Create variables.tf
	variablesContent := `variable "project_id" {
  type = string
}

variable "environment" {
  type = string
}

variable "labels" {
  default = {}
  type = map(string)
}`
	err = os.WriteFile(moduleDir+"/variables.tf", []byte(variablesContent), 0644)
	require.NoError(t, err)

	oldDirFlag := lintDirFlag
	oldAllFlag := lintAllFlag
	t.Cleanup(func() {
		lintDirFlag = oldDirFlag
		lintAllFlag = oldAllFlag
	})

	lintDirFlag = tmpDir
	lintAllFlag = true

	err = runLint(lintCmd, nil)
	require.NoError(t, err)
}

// Catalog generate tests

func TestRunCatalogGenerate_BadTemplate(t *testing.T) {
	oldTemplateFlag := templateFlag
	oldCatalogFlag := catalogFlag
	oldOutputFlag := outputFlag
	oldDryRunFlag := dryRunFlag
	t.Cleanup(func() {
		templateFlag = oldTemplateFlag
		catalogFlag = oldCatalogFlag
		outputFlag = oldOutputFlag
		dryRunFlag = oldDryRunFlag
	})

	templateFlag = "/nonexistent/template.hcl"
	catalogFlag = "/any"
	outputFlag = "/any"
	dryRunFlag = false

	err := runCatalogGenerate(catalogGenerateCmd, nil)
	assert.Error(t, err)
}

func TestRunCatalogGenerate_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	templatePath := tmpDir + "/template.hcl"
	catalogDir := tmpDir + "/catalog"
	outputDir := tmpDir + "/output"

	// Create template file
	templateContent := `stack "acme-service" {
  values = {
    name = "acme-service"
  }
}`
	err := os.WriteFile(templatePath, []byte(templateContent), 0644)
	require.NoError(t, err)

	// Create catalog structure
	err = os.MkdirAll(catalogDir+"/root", 0755)
	require.NoError(t, err)
	err = os.MkdirAll(catalogDir+"/project", 0755)
	require.NoError(t, err)

	// Create root config file
	rootConfigContent := `# root config`
	err = os.WriteFile(catalogDir+"/root/terragrunt-root.hcl", []byte(rootConfigContent), 0644)
	require.NoError(t, err)

	// Create output dir
	err = os.MkdirAll(outputDir, 0755)
	require.NoError(t, err)

	oldTemplateFlag := templateFlag
	oldCatalogFlag := catalogFlag
	oldOutputFlag := outputFlag
	oldDryRunFlag := dryRunFlag
	t.Cleanup(func() {
		templateFlag = oldTemplateFlag
		catalogFlag = oldCatalogFlag
		outputFlag = oldOutputFlag
		dryRunFlag = oldDryRunFlag
	})

	templateFlag = templatePath
	catalogFlag = catalogDir
	outputFlag = outputDir
	dryRunFlag = true

	err = runCatalogGenerate(catalogGenerateCmd, nil)
	require.NoError(t, err)

	// Verify output dir is empty (dry run wrote nothing)
	files, err := os.ReadDir(outputDir)
	require.NoError(t, err)
	assert.Equal(t, 0, len(files))
}

// State import tests

func TestRunStateImport_ConfigNotFound(t *testing.T) {
	oldConfigFlag := importConfigFlag
	oldDirFlag := importDirFlag
	oldVarFlags := importVarFlags
	oldDryRunFlag := importDryRunFlag
	t.Cleanup(func() {
		importConfigFlag = oldConfigFlag
		importDirFlag = oldDirFlag
		importVarFlags = oldVarFlags
		importDryRunFlag = oldDryRunFlag
	})

	importConfigFlag = "/nonexistent/config.yaml"
	importDirFlag = ""
	importVarFlags = nil
	importDryRunFlag = false

	err := runStateImport(stateImportCmd, nil)
	assert.Error(t, err)
}

func TestRunStateImport_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	// Create an empty config file (invalid)
	err := os.WriteFile(configPath, []byte(""), 0644)
	require.NoError(t, err)

	oldConfigFlag := importConfigFlag
	oldDirFlag := importDirFlag
	oldVarFlags := importVarFlags
	oldDryRunFlag := importDryRunFlag
	t.Cleanup(func() {
		importConfigFlag = oldConfigFlag
		importDirFlag = oldDirFlag
		importVarFlags = oldVarFlags
		importDryRunFlag = oldDryRunFlag
	})

	importConfigFlag = configPath
	importDirFlag = ""
	importVarFlags = nil
	importDryRunFlag = false

	err = runStateImport(stateImportCmd, nil)
	assert.Error(t, err)
}

func TestRunStateImport_BadVarFlag(t *testing.T) {
	oldConfigFlag := importConfigFlag
	oldVarFlags := importVarFlags
	t.Cleanup(func() {
		importConfigFlag = oldConfigFlag
		importVarFlags = oldVarFlags
	})

	importConfigFlag = "/any"
	importVarFlags = []string{"no-equals-sign"}

	err := runStateImport(stateImportCmd, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected key=value")
}

func TestRunStateScaffold_FullPath(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := tmpDir + "/output.yaml"

	// Save old function seams
	oldGeneratePlanJSONFn := generatePlanJSONFn
	oldCheckVersionFn := checkVersionFn
	oldCheckTerragruntVersionFn := checkTerragruntVersionFn
	t.Cleanup(func() {
		generatePlanJSONFn = oldGeneratePlanJSONFn
		checkVersionFn = oldCheckVersionFn
		checkTerragruntVersionFn = oldCheckTerragruntVersionFn
	})

	// Mock generatePlanJSONFn
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(`{"format_version":"1.0","resource_changes":[{"address":"aws_s3_bucket.acme","type":"aws_s3_bucket","change":{"actions":["create"],"after":{"bucket":"acme-bucket"}}}]}`), nil
	}
	checkVersionFn = func(ctx context.Context) error {
		return nil
	}
	checkTerragruntVersionFn = func(ctx context.Context) error {
		return nil
	}

	oldDirFlag := scaffoldDirFlag
	oldOutputFlag := scaffoldOutputFlag
	oldFetchRegistryFlag := scaffoldFetchRegistryFlag
	t.Cleanup(func() {
		scaffoldDirFlag = oldDirFlag
		scaffoldOutputFlag = oldOutputFlag
		scaffoldFetchRegistryFlag = oldFetchRegistryFlag
	})

	scaffoldDirFlag = tmpDir
	scaffoldOutputFlag = outputPath
	scaffoldFetchRegistryFlag = false

	err := runStateScaffold(stateScaffoldCmd, nil)
	require.NoError(t, err)

	// Verify output file was created and contains YAML with aws_s3_bucket
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "aws_s3_bucket")
}

func TestRunStateScaffold_NoResources(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := tmpDir + "/output.yaml"

	// Save old function seams
	oldGeneratePlanJSONFn := generatePlanJSONFn
	oldCheckVersionFn := checkVersionFn
	oldCheckTerragruntVersionFn := checkTerragruntVersionFn
	t.Cleanup(func() {
		generatePlanJSONFn = oldGeneratePlanJSONFn
		checkVersionFn = oldCheckVersionFn
		checkTerragruntVersionFn = oldCheckTerragruntVersionFn
	})

	// Mock generatePlanJSONFn to return plan with no creates
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(`{"format_version":"1.0","resource_changes":[{"address":"aws_s3_bucket.existing","type":"aws_s3_bucket","change":{"actions":["no-op"]}}]}`), nil
	}
	checkVersionFn = func(ctx context.Context) error {
		return nil
	}
	checkTerragruntVersionFn = func(ctx context.Context) error {
		return nil
	}

	oldDirFlag := scaffoldDirFlag
	oldOutputFlag := scaffoldOutputFlag
	oldFetchRegistryFlag := scaffoldFetchRegistryFlag
	t.Cleanup(func() {
		scaffoldDirFlag = oldDirFlag
		scaffoldOutputFlag = oldOutputFlag
		scaffoldFetchRegistryFlag = oldFetchRegistryFlag
	})

	scaffoldDirFlag = tmpDir
	scaffoldOutputFlag = outputPath
	scaffoldFetchRegistryFlag = false

	err := runStateScaffold(stateScaffoldCmd, nil)
	require.NoError(t, err)
}

func TestRunStateImport_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	// Create valid config YAML
	configContent := `types:
  aws_s3_bucket:
    id: "{{.bucket}}"`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Create .terraform dir to satisfy CheckInit
	err = os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	// Save old function seams
	oldCheckVersionFn := checkVersionFn
	oldGeneratePlanJSONFn := generatePlanJSONFn
	oldCheckStateFn := checkStateFn
	t.Cleanup(func() {
		checkVersionFn = oldCheckVersionFn
		generatePlanJSONFn = oldGeneratePlanJSONFn
		checkStateFn = oldCheckStateFn
	})

	// Mock seams
	checkVersionFn = func(ctx context.Context) error {
		return nil
	}
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(`{"format_version":"1.0","resource_changes":[{"address":"aws_s3_bucket.acme","type":"aws_s3_bucket","change":{"actions":["create"],"after":{"bucket":"acme-bucket"}}}]}`), nil
	}
	checkStateFn = func(ctx context.Context, workDir string, addrs []string, useTerragrunt bool) ([]string, error) {
		return nil, nil
	}

	oldConfigFlag := importConfigFlag
	oldDirFlag := importDirFlag
	oldVarFlags := importVarFlags
	oldDryRunFlag := importDryRunFlag
	oldForceFlag := importForceFlag
	t.Cleanup(func() {
		importConfigFlag = oldConfigFlag
		importDirFlag = oldDirFlag
		importVarFlags = oldVarFlags
		importDryRunFlag = oldDryRunFlag
		importForceFlag = oldForceFlag
	})

	importConfigFlag = configPath
	importDirFlag = tmpDir
	importVarFlags = nil
	importDryRunFlag = true
	importForceFlag = false

	err = runStateImport(stateImportCmd, nil)
	require.NoError(t, err)
}

func TestRunStateImport_NoCreates(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	// Create valid config YAML
	configContent := `types:
  aws_s3_bucket:
    id: "{{.bucket}}"`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Create .terraform dir to satisfy CheckInit
	err = os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	// Save old function seams
	oldCheckVersionFn := checkVersionFn
	oldGeneratePlanJSONFn := generatePlanJSONFn
	t.Cleanup(func() {
		checkVersionFn = oldCheckVersionFn
		generatePlanJSONFn = oldGeneratePlanJSONFn
	})

	// Mock seams
	checkVersionFn = func(ctx context.Context) error {
		return nil
	}
	// Plan with only no-op resources, no creates
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(`{"format_version":"1.0","resource_changes":[{"address":"aws_s3_bucket.existing","type":"aws_s3_bucket","change":{"actions":["no-op"]}}]}`), nil
	}

	oldConfigFlag := importConfigFlag
	oldDirFlag := importDirFlag
	oldVarFlags := importVarFlags
	oldDryRunFlag := importDryRunFlag
	t.Cleanup(func() {
		importConfigFlag = oldConfigFlag
		importDirFlag = oldDirFlag
		importVarFlags = oldVarFlags
		importDryRunFlag = oldDryRunFlag
	})

	importConfigFlag = configPath
	importDirFlag = tmpDir
	importVarFlags = nil
	importDryRunFlag = false

	err = runStateImport(stateImportCmd, nil)
	require.NoError(t, err)
}

func TestRunStateImport_AllManaged(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	// Create valid config YAML
	configContent := `types:
  aws_s3_bucket:
    id: "{{.bucket}}"`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Create .terraform dir to satisfy CheckInit
	err = os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	// Save old function seams
	oldCheckVersionFn := checkVersionFn
	oldGeneratePlanJSONFn := generatePlanJSONFn
	oldCheckStateFn := checkStateFn
	t.Cleanup(func() {
		checkVersionFn = oldCheckVersionFn
		generatePlanJSONFn = oldGeneratePlanJSONFn
		checkStateFn = oldCheckStateFn
	})

	// Mock seams
	checkVersionFn = func(ctx context.Context) error {
		return nil
	}
	// Plan with one create
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(`{"format_version":"1.0","resource_changes":[{"address":"aws_s3_bucket.acme","type":"aws_s3_bucket","change":{"actions":["create"],"after":{"bucket":"acme-bucket"}}}]}`), nil
	}
	// Mark all creates as already managed
	checkStateFn = func(ctx context.Context, workDir string, addrs []string, useTerragrunt bool) ([]string, error) {
		return addrs, nil
	}

	oldConfigFlag := importConfigFlag
	oldDirFlag := importDirFlag
	oldVarFlags := importVarFlags
	oldDryRunFlag := importDryRunFlag
	t.Cleanup(func() {
		importConfigFlag = oldConfigFlag
		importDirFlag = oldDirFlag
		importVarFlags = oldVarFlags
		importDryRunFlag = oldDryRunFlag
	})

	importConfigFlag = configPath
	importDirFlag = tmpDir
	importVarFlags = nil
	importDryRunFlag = false

	err = runStateImport(stateImportCmd, nil)
	require.NoError(t, err)
}

func TestRunStateScaffold_ParseError(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := tmpDir + "/output.yaml"

	// Save old function seams
	oldGeneratePlanJSONFn := generatePlanJSONFn
	oldCheckVersionFn := checkVersionFn
	oldCheckTerragruntVersionFn := checkTerragruntVersionFn
	t.Cleanup(func() {
		generatePlanJSONFn = oldGeneratePlanJSONFn
		checkVersionFn = oldCheckVersionFn
		checkTerragruntVersionFn = oldCheckTerragruntVersionFn
	})

	// Mock generatePlanJSONFn to return invalid JSON
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(`not valid json`), nil
	}
	checkVersionFn = func(ctx context.Context) error {
		return nil
	}
	checkTerragruntVersionFn = func(ctx context.Context) error {
		return nil
	}

	oldDirFlag := scaffoldDirFlag
	oldOutputFlag := scaffoldOutputFlag
	oldFetchRegistryFlag := scaffoldFetchRegistryFlag
	t.Cleanup(func() {
		scaffoldDirFlag = oldDirFlag
		scaffoldOutputFlag = oldOutputFlag
		scaffoldFetchRegistryFlag = oldFetchRegistryFlag
	})

	scaffoldDirFlag = tmpDir
	scaffoldOutputFlag = outputPath
	scaffoldFetchRegistryFlag = false

	err := runStateScaffold(stateScaffoldCmd, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse plan")
}

func TestRootCmd_Execute(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	os.Args = []string{"terranoodle", "--version"}

	err := Execute()
	require.NoError(t, err)
}

func TestRootCmd_VersionEmpty(t *testing.T) {
	oldVersion := Version
	oldFlag := versionFlag
	t.Cleanup(func() {
		Version = oldVersion
		versionFlag = oldFlag
	})

	Version = ""
	versionFlag = true

	err := rootCmd.RunE(rootCmd, nil)
	require.NoError(t, err)
}

func TestRunCatalogGenerate_Warnings(t *testing.T) {
	tmpDir := t.TempDir()
	templatePath := tmpDir + "/template.hcl"
	catalogDir := tmpDir + "/catalog"
	outputDir := tmpDir + "/output"

	// Create a template file that will generate warnings
	templateContent := `stack "acme-service" {
  values = {
    name = "acme-service"
  }
}`
	err := os.WriteFile(templatePath, []byte(templateContent), 0644)
	require.NoError(t, err)

	// Create catalog structure
	err = os.MkdirAll(catalogDir+"/root", 0755)
	require.NoError(t, err)
	err = os.MkdirAll(catalogDir+"/project", 0755)
	require.NoError(t, err)

	// Create root config file
	rootConfigContent := `# root config`
	err = os.WriteFile(catalogDir+"/root/terragrunt-root.hcl", []byte(rootConfigContent), 0644)
	require.NoError(t, err)

	// Create output dir
	err = os.MkdirAll(outputDir, 0755)
	require.NoError(t, err)

	oldTemplateFlag := templateFlag
	oldCatalogFlag := catalogFlag
	oldOutputFlag := outputFlag
	oldDryRunFlag := dryRunFlag
	t.Cleanup(func() {
		templateFlag = oldTemplateFlag
		catalogFlag = oldCatalogFlag
		outputFlag = oldOutputFlag
		dryRunFlag = oldDryRunFlag
	})

	templateFlag = templatePath
	catalogFlag = catalogDir
	outputFlag = outputDir
	dryRunFlag = false

	err = runCatalogGenerate(catalogGenerateCmd, nil)
	require.NoError(t, err)
}

func TestRunCatalogGenerate_GeneratorErrors(t *testing.T) {
	tmpDir := t.TempDir()
	templatePath := tmpDir + "/template.hcl"
	catalogDir := tmpDir + "/catalog"
	// Use a non-writable directory to trigger generator errors
	outputDir := tmpDir + "/readonly"
	err := os.MkdirAll(outputDir, 0444)
	require.NoError(t, err)

	// Create a template file
	templateContent := `stack "acme-service" {
  values = {
    name = "acme-service"
  }
}`
	err = os.WriteFile(templatePath, []byte(templateContent), 0644)
	require.NoError(t, err)

	// Create catalog structure
	err = os.MkdirAll(catalogDir+"/root", 0755)
	require.NoError(t, err)
	err = os.MkdirAll(catalogDir+"/project", 0755)
	require.NoError(t, err)

	// Create root config file
	rootConfigContent := `# root config`
	err = os.WriteFile(catalogDir+"/root/terragrunt-root.hcl", []byte(rootConfigContent), 0644)
	require.NoError(t, err)

	oldTemplateFlag := templateFlag
	oldCatalogFlag := catalogFlag
	oldOutputFlag := outputFlag
	oldDryRunFlag := dryRunFlag
	t.Cleanup(func() {
		templateFlag = oldTemplateFlag
		catalogFlag = oldCatalogFlag
		outputFlag = oldOutputFlag
		dryRunFlag = oldDryRunFlag
		// Restore write permissions for cleanup
		_ = os.Chmod(outputDir, 0755)
	})

	templateFlag = templatePath
	catalogFlag = catalogDir
	outputFlag = outputDir
	dryRunFlag = false

	err = runCatalogGenerate(catalogGenerateCmd, nil)
	assert.Error(t, err)
}

func TestRunStateImport_FullPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	// Create valid config YAML with type mapping
	configContent := `types:
  aws_s3_bucket:
    id: "{{.bucket}}"`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Create .terraform dir to satisfy CheckInit
	err = os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	// Save old function seams
	oldCheckVersionFn := checkVersionFn
	oldGeneratePlanJSONFn := generatePlanJSONFn
	oldCheckStateFn := checkStateFn
	oldCheckInitFn := checkInitFn
	oldApplyFn := applyFn
	t.Cleanup(func() {
		checkVersionFn = oldCheckVersionFn
		generatePlanJSONFn = oldGeneratePlanJSONFn
		checkStateFn = oldCheckStateFn
		checkInitFn = oldCheckInitFn
		applyFn = oldApplyFn
	})

	// Mock seams
	checkVersionFn = func(ctx context.Context) error {
		return nil
	}
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(`{"format_version":"1.0","resource_changes":[{"address":"aws_s3_bucket.acme","type":"aws_s3_bucket","change":{"actions":["create"],"after":{"bucket":"acme-bucket"}}}]}`), nil
	}
	checkStateFn = func(ctx context.Context, workDir string, addrs []string, useTerragrunt bool) ([]string, error) {
		return nil, nil
	}
	checkInitFn = func(workDir string) error {
		return nil
	}
	applyFn = func(ctx context.Context, workDir string, useTerragrunt bool) error {
		return nil
	}

	oldConfigFlag := importConfigFlag
	oldDirFlag := importDirFlag
	oldVarFlags := importVarFlags
	oldDryRunFlag := importDryRunFlag
	oldForceFlag := importForceFlag
	t.Cleanup(func() {
		importConfigFlag = oldConfigFlag
		importDirFlag = oldDirFlag
		importVarFlags = oldVarFlags
		importDryRunFlag = oldDryRunFlag
		importForceFlag = oldForceFlag
	})

	importConfigFlag = configPath
	importDirFlag = tmpDir
	importVarFlags = nil
	importDryRunFlag = false
	importForceFlag = true

	err = runStateImport(stateImportCmd, nil)
	require.NoError(t, err)
}

func TestRunStateImport_CheckInitError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	// Create valid config YAML
	configContent := `types:
  aws_s3_bucket:
    id: "{{.bucket}}"`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Don't create .terraform dir to trigger CheckInit error

	// Save old function seams
	oldCheckVersionFn := checkVersionFn
	oldGeneratePlanJSONFn := generatePlanJSONFn
	oldCheckInitFn := checkInitFn
	t.Cleanup(func() {
		checkVersionFn = oldCheckVersionFn
		generatePlanJSONFn = oldGeneratePlanJSONFn
		checkInitFn = oldCheckInitFn
	})

	// Mock seams
	checkVersionFn = func(ctx context.Context) error {
		return nil
	}
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(`{"format_version":"1.0","resource_changes":[]}`), nil
	}
	checkInitFn = func(workDir string) error {
		return fmt.Errorf("terraform not initialized")
	}

	oldConfigFlag := importConfigFlag
	oldDirFlag := importDirFlag
	oldVarFlags := importVarFlags
	oldDryRunFlag := importDryRunFlag
	t.Cleanup(func() {
		importConfigFlag = oldConfigFlag
		importDirFlag = oldDirFlag
		importVarFlags = oldVarFlags
		importDryRunFlag = oldDryRunFlag
	})

	importConfigFlag = configPath
	importDirFlag = tmpDir
	importVarFlags = nil
	importDryRunFlag = false

	err = runStateImport(stateImportCmd, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestRunLint_DefaultDir(t *testing.T) {
	oldDirFlag := lintDirFlag
	oldAllFlag := lintAllFlag
	t.Cleanup(func() {
		lintDirFlag = oldDirFlag
		lintAllFlag = oldAllFlag
	})

	lintDirFlag = ""
	lintAllFlag = false

	err := runLint(lintCmd, nil)
	require.NoError(t, err)
}

func TestRootCmd_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	err := rootCmd.PersistentPreRunE(rootCmd, nil)
	require.NoError(t, err)
}

func TestRunStateImport_WithVarOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	// Config with vars and type using a var
	configContent := `vars:
  region: us-east-1
types:
  aws_s3_bucket:
    id: "{{.bucket}}"`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	oldCheckVersionFn := checkVersionFn
	oldGeneratePlanJSONFn := generatePlanJSONFn
	oldCheckStateFn := checkStateFn
	oldCheckInitFn := checkInitFn
	t.Cleanup(func() {
		checkVersionFn = oldCheckVersionFn
		generatePlanJSONFn = oldGeneratePlanJSONFn
		checkStateFn = oldCheckStateFn
		checkInitFn = oldCheckInitFn
	})

	checkVersionFn = func(ctx context.Context) error { return nil }
	checkInitFn = func(workDir string) error { return nil }
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(`{"format_version":"1.0","resource_changes":[{"address":"aws_s3_bucket.acme","type":"aws_s3_bucket","change":{"actions":["create"],"after":{"bucket":"acme-bucket"}}}]}`), nil
	}
	checkStateFn = func(ctx context.Context, workDir string, addrs []string, useTerragrunt bool) ([]string, error) {
		return nil, nil
	}

	oldConfigFlag := importConfigFlag
	oldDirFlag := importDirFlag
	oldVarFlags := importVarFlags
	oldDryRunFlag := importDryRunFlag
	oldForceFlag := importForceFlag
	t.Cleanup(func() {
		importConfigFlag = oldConfigFlag
		importDirFlag = oldDirFlag
		importVarFlags = oldVarFlags
		importDryRunFlag = oldDryRunFlag
		importForceFlag = oldForceFlag
	})

	importConfigFlag = configPath
	importDirFlag = tmpDir
	importVarFlags = []string{"region=us-west-2"}
	importDryRunFlag = true
	importForceFlag = false

	err = runStateImport(stateImportCmd, nil)
	require.NoError(t, err)
}

func TestRunCatalogGenerate_BadCatalog(t *testing.T) {
	tmpDir := t.TempDir()
	templatePath := tmpDir + "/template.hcl"

	templateContent := `stack "acme-service" {
  values = {
    name = "acme-service"
  }
}`
	err := os.WriteFile(templatePath, []byte(templateContent), 0644)
	require.NoError(t, err)

	oldTemplateFlag := templateFlag
	oldCatalogFlag := catalogFlag
	oldOutputFlag := outputFlag
	oldDryRunFlag := dryRunFlag
	t.Cleanup(func() {
		templateFlag = oldTemplateFlag
		catalogFlag = oldCatalogFlag
		outputFlag = oldOutputFlag
		dryRunFlag = oldDryRunFlag
	})

	templateFlag = templatePath
	catalogFlag = "/nonexistent/catalog"
	outputFlag = tmpDir
	dryRunFlag = false

	err = runCatalogGenerate(catalogGenerateCmd, nil)
	assert.Error(t, err)
}

func TestRunStateImport_GeneratePlanError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	configContent := `types:
  aws_s3_bucket:
    id: "{{.bucket}}"`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	oldCheckVersionFn := checkVersionFn
	oldCheckInitFn := checkInitFn
	oldGeneratePlanJSONFn := generatePlanJSONFn
	t.Cleanup(func() {
		checkVersionFn = oldCheckVersionFn
		checkInitFn = oldCheckInitFn
		generatePlanJSONFn = oldGeneratePlanJSONFn
	})

	checkVersionFn = func(ctx context.Context) error { return nil }
	checkInitFn = func(workDir string) error { return nil }
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return nil, fmt.Errorf("plan generation failed")
	}

	oldConfigFlag := importConfigFlag
	oldDirFlag := importDirFlag
	oldVarFlags := importVarFlags
	oldDryRunFlag := importDryRunFlag
	t.Cleanup(func() {
		importConfigFlag = oldConfigFlag
		importDirFlag = oldDirFlag
		importVarFlags = oldVarFlags
		importDryRunFlag = oldDryRunFlag
	})

	importConfigFlag = configPath
	importDirFlag = tmpDir
	importVarFlags = nil
	importDryRunFlag = false

	err = runStateImport(stateImportCmd, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plan generation failed")
}

func TestRunStateImport_ParsePlanError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	configContent := `types:
  aws_s3_bucket:
    id: "{{.bucket}}"`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	oldCheckVersionFn := checkVersionFn
	oldCheckInitFn := checkInitFn
	oldGeneratePlanJSONFn := generatePlanJSONFn
	t.Cleanup(func() {
		checkVersionFn = oldCheckVersionFn
		checkInitFn = oldCheckInitFn
		generatePlanJSONFn = oldGeneratePlanJSONFn
	})

	checkVersionFn = func(ctx context.Context) error { return nil }
	checkInitFn = func(workDir string) error { return nil }
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(`not valid json`), nil
	}

	oldConfigFlag := importConfigFlag
	oldDirFlag := importDirFlag
	oldVarFlags := importVarFlags
	oldDryRunFlag := importDryRunFlag
	t.Cleanup(func() {
		importConfigFlag = oldConfigFlag
		importDirFlag = oldDirFlag
		importVarFlags = oldVarFlags
		importDryRunFlag = oldDryRunFlag
	})

	importConfigFlag = configPath
	importDirFlag = tmpDir
	importVarFlags = nil
	importDryRunFlag = false

	err = runStateImport(stateImportCmd, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse plan")
}

func TestRunStateScaffold_GeneratePlanError(t *testing.T) {
	tmpDir := t.TempDir()

	oldGeneratePlanJSONFn := generatePlanJSONFn
	oldCheckVersionFn := checkVersionFn
	oldCheckTerragruntVersionFn := checkTerragruntVersionFn
	t.Cleanup(func() {
		generatePlanJSONFn = oldGeneratePlanJSONFn
		checkVersionFn = oldCheckVersionFn
		checkTerragruntVersionFn = oldCheckTerragruntVersionFn
	})

	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return nil, fmt.Errorf("plan generation failed")
	}
	checkVersionFn = func(ctx context.Context) error { return nil }
	checkTerragruntVersionFn = func(ctx context.Context) error { return nil }

	oldDirFlag := scaffoldDirFlag
	oldOutputFlag := scaffoldOutputFlag
	oldFetchRegistryFlag := scaffoldFetchRegistryFlag
	t.Cleanup(func() {
		scaffoldDirFlag = oldDirFlag
		scaffoldOutputFlag = oldOutputFlag
		scaffoldFetchRegistryFlag = oldFetchRegistryFlag
	})

	scaffoldDirFlag = tmpDir
	scaffoldOutputFlag = ""
	scaffoldFetchRegistryFlag = false

	err := runStateScaffold(stateScaffoldCmd, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plan generation failed")
}

func TestRunStateScaffold_OutputToStdout(t *testing.T) {
	tmpDir := t.TempDir()

	// Save old function seam
	oldGeneratePlanJSONFn := generatePlanJSONFn
	oldCheckVersionFn := checkVersionFn
	oldCheckTerragruntVersionFn := checkTerragruntVersionFn
	t.Cleanup(func() {
		generatePlanJSONFn = oldGeneratePlanJSONFn
		checkVersionFn = oldCheckVersionFn
		checkTerragruntVersionFn = oldCheckTerragruntVersionFn
	})

	// Mock generatePlanJSONFn
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(`{"format_version":"1.0","resource_changes":[{"address":"aws_s3_bucket.acme","type":"aws_s3_bucket","change":{"actions":["create"],"after":{"bucket":"acme-bucket"}}}]}`), nil
	}
	checkVersionFn = func(ctx context.Context) error { return nil }
	checkTerragruntVersionFn = func(ctx context.Context) error { return nil }

	oldDirFlag := scaffoldDirFlag
	oldOutputFlag := scaffoldOutputFlag
	oldFetchRegistryFlag := scaffoldFetchRegistryFlag
	t.Cleanup(func() {
		scaffoldDirFlag = oldDirFlag
		scaffoldOutputFlag = oldOutputFlag
		scaffoldFetchRegistryFlag = oldFetchRegistryFlag
	})

	scaffoldDirFlag = tmpDir
	scaffoldOutputFlag = ""
	scaffoldFetchRegistryFlag = false

	err := runStateScaffold(stateScaffoldCmd, nil)
	require.NoError(t, err)
}

// State rename tests

func saveRenameFlags(t *testing.T) {
	oldMoved := renameMovedFlag
	oldMv := renameMvFlag
	oldApply := renameApplyFlag
	oldDir := renameDirFlag
	oldPlan := renamePlanFlag
	oldOutput := renameOutputFlag
	oldForce := renameForceFlag
	t.Cleanup(func() {
		renameMovedFlag = oldMoved
		renameMvFlag = oldMv
		renameApplyFlag = oldApply
		renameDirFlag = oldDir
		renamePlanFlag = oldPlan
		renameOutputFlag = oldOutput
		renameForceFlag = oldForce
	})
}

func saveRenameSeams(t *testing.T) {
	oldGeneratePlan := generatePlanJSONFn
	oldCheckVersion := checkVersionFn
	oldCheckTgVersion := checkTerragruntVersionFn
	oldCheckInit := checkInitFn
	oldStateMv := stateMvFn
	oldConfirm := confirmCandidatesFn
	t.Cleanup(func() {
		generatePlanJSONFn = oldGeneratePlan
		checkVersionFn = oldCheckVersion
		checkTerragruntVersionFn = oldCheckTgVersion
		checkInitFn = oldCheckInit
		stateMvFn = oldStateMv
		confirmCandidatesFn = oldConfirm
	})
	checkVersionFn = func(ctx context.Context) error { return nil }
	checkTerragruntVersionFn = func(ctx context.Context) error { return nil }
	checkInitFn = func(workDir string) error { return nil }
}

const renamePlanWithPreviousAddress = `{
  "format_version": "1.0",
  "resource_changes": [
    {
      "address": "module.storage.aws_s3_bucket.data",
      "previous_address": "aws_s3_bucket.data",
      "type": "aws_s3_bucket",
      "change": {"actions": ["no-op"]}
    }
  ]
}`

const renamePlanWithDestroyCreate = `{
  "format_version": "1.0",
  "resource_changes": [
    {
      "address": "aws_s3_bucket.old_name",
      "type": "aws_s3_bucket",
      "change": {"actions": ["delete"]}
    },
    {
      "address": "aws_s3_bucket.new_name",
      "type": "aws_s3_bucket",
      "change": {"actions": ["create"]}
    }
  ]
}`

const renamePlanNoRenames = `{
  "format_version": "1.0",
  "resource_changes": [
    {
      "address": "aws_s3_bucket.example",
      "type": "aws_s3_bucket",
      "change": {"actions": ["no-op"]}
    }
  ]
}`

func TestRunStateRename_NoFlags(t *testing.T) {
	saveRenameFlags(t)
	renameMovedFlag = false
	renameMvFlag = false

	err := runStateRename(stateRenameCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "one of --moved or --mv is required")
}

func TestRunStateRename_BothFlags(t *testing.T) {
	saveRenameFlags(t)
	renameMovedFlag = true
	renameMvFlag = true

	err := runStateRename(stateRenameCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestRunStateRename_Moved_Preview(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = true
	renameMvFlag = false
	renameApplyFlag = false
	renameDirFlag = tmpDir
	renamePlanFlag = ""
	renameOutputFlag = ""
	renameForceFlag = false

	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(renamePlanWithPreviousAddress), nil
	}

	err = runStateRename(stateRenameCmd, nil)
	require.NoError(t, err)
}

func TestRunStateRename_Moved_Apply(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = true
	renameMvFlag = false
	renameApplyFlag = true
	renameDirFlag = tmpDir
	renamePlanFlag = ""
	renameOutputFlag = ""
	renameForceFlag = false

	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(renamePlanWithPreviousAddress), nil
	}

	err = runStateRename(stateRenameCmd, nil)
	require.NoError(t, err)

	// Verify moved.tf was written
	data, readErr := os.ReadFile(filepath.Join(tmpDir, "moved.tf"))
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "moved {")
	assert.Contains(t, string(data), "from = aws_s3_bucket.data")
	assert.Contains(t, string(data), "to   = module.storage.aws_s3_bucket.data")
}

func TestRunStateRename_Moved_CustomOutput(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)
	customPath := filepath.Join(tmpDir, "custom-moved.tf")

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = true
	renameMvFlag = false
	renameApplyFlag = true
	renameDirFlag = tmpDir
	renamePlanFlag = ""
	renameOutputFlag = customPath
	renameForceFlag = false

	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(renamePlanWithPreviousAddress), nil
	}

	err = runStateRename(stateRenameCmd, nil)
	require.NoError(t, err)

	_, statErr := os.Stat(customPath)
	assert.NoError(t, statErr)
}

func TestRunStateRename_Mv_Preview(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = false
	renameMvFlag = true
	renameApplyFlag = false
	renameDirFlag = tmpDir
	renamePlanFlag = ""

	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(renamePlanWithPreviousAddress), nil
	}

	err = runStateRename(stateRenameCmd, nil)
	require.NoError(t, err)
}

func TestRunStateRename_Mv_Apply(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = false
	renameMvFlag = true
	renameApplyFlag = true
	renameDirFlag = tmpDir
	renamePlanFlag = ""

	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(renamePlanWithPreviousAddress), nil
	}

	var mvCalls []string
	stateMvFn = func(ctx context.Context, workDir, from, to string, useTerragrunt bool) error {
		mvCalls = append(mvCalls, fmt.Sprintf("%s -> %s", from, to))
		return nil
	}

	err = runStateRename(stateRenameCmd, nil)
	require.NoError(t, err)
	require.Len(t, mvCalls, 1)
	assert.Equal(t, "aws_s3_bucket.data -> module.storage.aws_s3_bucket.data", mvCalls[0])
}

func TestRunStateRename_NoRenames(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = true
	renameMvFlag = false
	renameApplyFlag = false
	renameDirFlag = tmpDir
	renamePlanFlag = ""

	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(renamePlanNoRenames), nil
	}

	err = runStateRename(stateRenameCmd, nil)
	require.NoError(t, err)
}

func TestRunStateRename_PlanFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	planPath := filepath.Join(tmpDir, "plan.json")
	err = os.WriteFile(planPath, []byte(renamePlanWithPreviousAddress), 0644)
	require.NoError(t, err)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = true
	renameMvFlag = false
	renameApplyFlag = false
	renameDirFlag = tmpDir
	renamePlanFlag = planPath

	err = runStateRename(stateRenameCmd, nil)
	require.NoError(t, err)
}

func TestRunStateRename_DestroyCreateCandidates(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = true
	renameMvFlag = false
	renameApplyFlag = true
	renameDirFlag = tmpDir
	renamePlanFlag = ""

	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(renamePlanWithDestroyCreate), nil
	}

	confirmCandidatesFn = func(candidates []rename.Candidate, autoConfirm bool) ([]rename.RenamePair, error) {
		return []rename.RenamePair{
			{From: "aws_s3_bucket.old_name", To: "aws_s3_bucket.new_name"},
		}, nil
	}

	err = runStateRename(stateRenameCmd, nil)
	require.NoError(t, err)

	data, readErr := os.ReadFile(filepath.Join(tmpDir, "moved.tf"))
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "from = aws_s3_bucket.old_name")
	assert.Contains(t, string(data), "to   = aws_s3_bucket.new_name")
}

func TestRunStateRename_MvApplyAutoConfirm(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = false
	renameMvFlag = true
	renameApplyFlag = true
	renameDirFlag = tmpDir
	renamePlanFlag = ""

	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(renamePlanWithDestroyCreate), nil
	}

	confirmCalled := false
	confirmCandidatesFn = func(candidates []rename.Candidate, autoConfirm bool) ([]rename.RenamePair, error) {
		confirmCalled = true
		assert.True(t, autoConfirm, "autoConfirm should be true when renameApplyFlag is true")
		return []rename.RenamePair{
			{From: "aws_s3_bucket.old_name", To: "aws_s3_bucket.new_name"},
		}, nil
	}

	stateMvFn = func(ctx context.Context, workDir, from, to string, useTerragrunt bool) error {
		assert.Equal(t, "aws_s3_bucket.old_name", from)
		assert.Equal(t, "aws_s3_bucket.new_name", to)
		return nil
	}

	err = runStateRename(stateRenameCmd, nil)
	require.NoError(t, err)
	assert.True(t, confirmCalled, "confirmCandidatesFn should have been called")
}

func TestRunStateRename_MvApplyTerragrunt(t *testing.T) {
	tmpDir := makeTerragruntFixture(t)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = false
	renameMvFlag = true
	renameApplyFlag = true
	renameDirFlag = tmpDir
	renamePlanFlag = ""

	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(renamePlanWithPreviousAddress), nil
	}

	var capturedWorkDir string
	var capturedUseTerragrunt bool
	stateMvFn = func(ctx context.Context, workDir, from, to string, useTerragrunt bool) error {
		capturedWorkDir = workDir
		capturedUseTerragrunt = useTerragrunt
		return nil
	}

	err := runStateRename(stateRenameCmd, nil)
	require.NoError(t, err)
	assert.True(t, capturedUseTerragrunt, "useTerragrunt should be true when .terragrunt-cache is present")
	assert.Equal(t, tmpDir, capturedWorkDir, "workDir should be the project root, not the cache directory")
}

func TestRunStateRename_PlanFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = true
	renameMvFlag = false
	renameDirFlag = tmpDir
	renamePlanFlag = "/nonexistent/plan.json"

	err = runStateRename(stateRenameCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read plan")
}

// makeTerragruntFixture creates a temporary directory with a .terragrunt-cache structure.
func makeTerragruntFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, ".terragrunt-cache", "abc", "def", ".terraform"), 0755)
	require.NoError(t, err)
	return dir
}

// State import error tests

func TestRunStateImport_VersionError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.hcl")
	err := os.WriteFile(configPath, []byte("types:\n  aws_s3_bucket:\n    id: \"{{.bucket}}\""), 0644)
	require.NoError(t, err)

	oldConfig := importConfigFlag
	oldDir := importDirFlag
	oldCheckVersion := checkVersionFn
	t.Cleanup(func() {
		importConfigFlag = oldConfig
		importDirFlag = oldDir
		checkVersionFn = oldCheckVersion
	})

	importConfigFlag = configPath
	importDirFlag = tmpDir
	checkVersionFn = func(ctx context.Context) error {
		return fmt.Errorf("version check failed")
	}

	err = runStateImport(stateImportCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version check failed")
}

func TestRunStateImport_TerragruntVersionError(t *testing.T) {
	tmpDir := makeTerragruntFixture(t)
	configPath := filepath.Join(tmpDir, "config.hcl")
	err := os.WriteFile(configPath, []byte("types:\n  aws_s3_bucket:\n    id: \"{{.bucket}}\""), 0644)
	require.NoError(t, err)

	oldConfig := importConfigFlag
	oldDir := importDirFlag
	oldCheckVersion := checkVersionFn
	oldCheckTgVersion := checkTerragruntVersionFn
	t.Cleanup(func() {
		importConfigFlag = oldConfig
		importDirFlag = oldDir
		checkVersionFn = oldCheckVersion
		checkTerragruntVersionFn = oldCheckTgVersion
	})

	importConfigFlag = configPath
	importDirFlag = tmpDir
	checkVersionFn = func(ctx context.Context) error { return nil }
	checkTerragruntVersionFn = func(ctx context.Context) error {
		return fmt.Errorf("terragrunt version check failed")
	}

	err = runStateImport(stateImportCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terragrunt version check failed")
}

func TestRunStateImport_TerragruntFindCacheError(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terragrunt-cache"), 0755)
	require.NoError(t, err)

	configPath := filepath.Join(tmpDir, "config.hcl")
	err = os.WriteFile(configPath, []byte("types:\n  aws_s3_bucket:\n    id: \"{{.bucket}}\""), 0644)
	require.NoError(t, err)

	oldConfig := importConfigFlag
	oldDir := importDirFlag
	oldCheckVersion := checkVersionFn
	oldCheckTgVersion := checkTerragruntVersionFn
	t.Cleanup(func() {
		importConfigFlag = oldConfig
		importDirFlag = oldDir
		checkVersionFn = oldCheckVersion
		checkTerragruntVersionFn = oldCheckTgVersion
	})

	importConfigFlag = configPath
	importDirFlag = tmpDir
	checkVersionFn = func(ctx context.Context) error { return nil }
	checkTerragruntVersionFn = func(ctx context.Context) error { return nil }

	err = runStateImport(stateImportCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initialised")
}

// State scaffold error tests

func TestRunStateScaffold_VersionError(t *testing.T) {
	oldCheckVersion := checkVersionFn
	t.Cleanup(func() {
		checkVersionFn = oldCheckVersion
	})

	checkVersionFn = func(ctx context.Context) error {
		return fmt.Errorf("version check failed")
	}

	err := runStateScaffold(stateScaffoldCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version check failed")
}

func TestRunStateScaffold_TerragruntVersionError(t *testing.T) {
	tmpDir := makeTerragruntFixture(t)

	oldDir := scaffoldDirFlag
	oldCheckVersion := checkVersionFn
	oldCheckTgVersion := checkTerragruntVersionFn
	t.Cleanup(func() {
		scaffoldDirFlag = oldDir
		checkVersionFn = oldCheckVersion
		checkTerragruntVersionFn = oldCheckTgVersion
	})

	scaffoldDirFlag = tmpDir
	checkVersionFn = func(ctx context.Context) error { return nil }
	checkTerragruntVersionFn = func(ctx context.Context) error {
		return fmt.Errorf("terragrunt version check failed")
	}

	err := runStateScaffold(stateScaffoldCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terragrunt version check failed")
}

func TestRunStateScaffold_FindCacheError(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terragrunt-cache"), 0755)
	require.NoError(t, err)

	oldDir := scaffoldDirFlag
	oldCheckVersion := checkVersionFn
	oldCheckTgVersion := checkTerragruntVersionFn
	t.Cleanup(func() {
		scaffoldDirFlag = oldDir
		checkVersionFn = oldCheckVersion
		checkTerragruntVersionFn = oldCheckTgVersion
	})

	scaffoldDirFlag = tmpDir
	checkVersionFn = func(ctx context.Context) error { return nil }
	checkTerragruntVersionFn = func(ctx context.Context) error { return nil }

	err = runStateScaffold(stateScaffoldCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initialised")
}

// State rename error tests

func TestRunStateRename_VersionError(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = true
	renameDirFlag = tmpDir

	checkVersionFn = func(ctx context.Context) error {
		return fmt.Errorf("version check failed")
	}

	err = runStateRename(stateRenameCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version check failed")
}

func TestRunStateRename_CheckInitError(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = true
	renameDirFlag = tmpDir

	checkInitFn = func(workDir string) error {
		return fmt.Errorf("init check failed")
	}

	err = runStateRename(stateRenameCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init check failed")
}

func TestRunStateRename_GeneratePlanError(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = true
	renameDirFlag = tmpDir

	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return nil, fmt.Errorf("plan generation failed")
	}

	err = runStateRename(stateRenameCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan generation failed")
}

func TestRunStateRename_ParsePlanError(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = true
	renameDirFlag = tmpDir

	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte("invalid json"), nil
	}

	err = runStateRename(stateRenameCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestRunStateRename_ConfirmCandidatesError(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMovedFlag = true
	renameDirFlag = tmpDir

	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(renamePlanWithDestroyCreate), nil
	}

	confirmCandidatesFn = func(candidates []rename.Candidate, autoConfirm bool) ([]rename.RenamePair, error) {
		return nil, fmt.Errorf("confirm failed")
	}

	err = runStateRename(stateRenameCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "confirm failed")
}

func TestRunStateRename_MvApply_StateMvError(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.MkdirAll(filepath.Join(tmpDir, ".terraform"), 0755)
	require.NoError(t, err)

	saveRenameFlags(t)
	saveRenameSeams(t)

	renameMvFlag = true
	renameApplyFlag = true
	renameDirFlag = tmpDir

	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(renamePlanWithPreviousAddress), nil
	}

	stateMvFn = func(ctx context.Context, workDir, from, to string, useTerragrunt bool) error {
		return fmt.Errorf("state mv failed")
	}

	err = runStateRename(stateRenameCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "state mv failed")
}
