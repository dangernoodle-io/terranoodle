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
	oldRecursiveFlag := lintRecursiveFlag
	t.Cleanup(func() {
		lintDirFlag = oldDirFlag
		lintRecursiveFlag = oldRecursiveFlag
	})

	lintDirFlag = configDir
	lintRecursiveFlag = false

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
	oldRecursiveFlag := lintRecursiveFlag
	t.Cleanup(func() {
		lintDirFlag = oldDirFlag
		lintRecursiveFlag = oldRecursiveFlag
	})

	lintDirFlag = configDir
	lintRecursiveFlag = false

	err = runLint(lintCmd, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "issue")
}

func TestRunLint_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	oldDirFlag := lintDirFlag
	oldRecursiveFlag := lintRecursiveFlag
	t.Cleanup(func() {
		lintDirFlag = oldDirFlag
		lintRecursiveFlag = oldRecursiveFlag
	})

	lintDirFlag = tmpDir
	lintRecursiveFlag = false

	err := runLint(lintCmd, nil)
	require.NoError(t, err)
}

func TestRunLint_Recursive(t *testing.T) {
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
	oldRecursiveFlag := lintRecursiveFlag
	t.Cleanup(func() {
		lintDirFlag = oldDirFlag
		lintRecursiveFlag = oldRecursiveFlag
	})

	lintDirFlag = tmpDir
	lintRecursiveFlag = true

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

	// Save old function seam
	oldGeneratePlanJSONFn := generatePlanJSONFn
	t.Cleanup(func() { generatePlanJSONFn = oldGeneratePlanJSONFn })

	// Mock generatePlanJSONFn
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(`{"format_version":"1.0","resource_changes":[{"address":"aws_s3_bucket.acme","type":"aws_s3_bucket","change":{"actions":["create"],"after":{"bucket":"acme-bucket"}}}]}`), nil
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

	// Save old function seam
	oldGeneratePlanJSONFn := generatePlanJSONFn
	t.Cleanup(func() { generatePlanJSONFn = oldGeneratePlanJSONFn })

	// Mock generatePlanJSONFn to return plan with no creates
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(`{"format_version":"1.0","resource_changes":[{"address":"aws_s3_bucket.existing","type":"aws_s3_bucket","change":{"actions":["no-op"]}}]}`), nil
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
	checkVersionFn = func(ctx context.Context, workDir string) error {
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
	checkVersionFn = func(ctx context.Context, workDir string) error {
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
	checkVersionFn = func(ctx context.Context, workDir string) error {
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

	// Save old function seam
	oldGeneratePlanJSONFn := generatePlanJSONFn
	t.Cleanup(func() { generatePlanJSONFn = oldGeneratePlanJSONFn })

	// Mock generatePlanJSONFn to return invalid JSON
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(`not valid json`), nil
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

	os.Args = []string{"terratools", "--version"}

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
	checkVersionFn = func(ctx context.Context, workDir string) error {
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
	checkVersionFn = func(ctx context.Context, workDir string) error {
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
	oldRecursiveFlag := lintRecursiveFlag
	t.Cleanup(func() {
		lintDirFlag = oldDirFlag
		lintRecursiveFlag = oldRecursiveFlag
	})

	lintDirFlag = ""
	lintRecursiveFlag = false

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

	checkVersionFn = func(ctx context.Context, workDir string) error { return nil }
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

	checkVersionFn = func(ctx context.Context, workDir string) error { return nil }
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

	checkVersionFn = func(ctx context.Context, workDir string) error { return nil }
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
	t.Cleanup(func() { generatePlanJSONFn = oldGeneratePlanJSONFn })

	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return nil, fmt.Errorf("plan generation failed")
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
	t.Cleanup(func() { generatePlanJSONFn = oldGeneratePlanJSONFn })

	// Mock generatePlanJSONFn
	generatePlanJSONFn = func(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
		return []byte(`{"format_version":"1.0","resource_changes":[{"address":"aws_s3_bucket.acme","type":"aws_s3_bucket","change":{"actions":["create"],"after":{"bucket":"acme-bucket"}}}]}`), nil
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
	scaffoldOutputFlag = ""
	scaffoldFetchRegistryFlag = false

	err := runStateScaffold(stateScaffoldCmd, nil)
	require.NoError(t, err)
}
