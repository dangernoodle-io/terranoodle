package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"dangernoodle.io/terratools/internal/catalog/catalog"
	"dangernoodle.io/terratools/internal/catalog/hclparse"
)

// minimalLayout returns a catalog Layout with no services suitable for
// tests that only exercise template-level validation.
func minimalLayout(t *testing.T) *catalog.Layout {
	t.Helper()
	dir := t.TempDir()

	// Create the minimum directory structure required for a non-nil Layout.
	rootDir := filepath.Join(dir, "root")
	require.NoError(t, os.MkdirAll(rootDir, 0o755))
	rootConfig := filepath.Join(rootDir, "terragrunt-root.hcl")
	require.NoError(t, os.WriteFile(rootConfig, []byte(`# root config`), 0o644))
	projectDir := filepath.Join(dir, "project")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	return &catalog.Layout{
		RootConfig: rootConfig,
		ProjectDir: projectDir,
		Services:   make(map[string]catalog.Service),
		Config:     &catalog.CatalogConfig{},
	}
}

// buildTemplateDef creates a TemplateDef with a single stack having the provided
// values. RawValues is left nil since we don't need cross-template references in tests.
func buildTemplateDef(name string, values map[string]cty.Value) *hclparse.TemplateDef {
	return &hclparse.TemplateDef{
		Stacks: []hclparse.UnitDef{
			{
				Name:      name,
				Values:    values,
				RawValues: nil,
			},
		},
	}
}

func TestGenerate_DryRun(t *testing.T) {
	layout := minimalLayout(t)
	def := buildTemplateDef("my-service", map[string]cty.Value{
		"env": cty.StringVal("prod"),
	})

	outputDir := t.TempDir()
	errs, err := Generate(&Config{
		TemplateDef: def,
		Catalog:     layout,
		OutputDir:   outputDir,
		DryRun:      true,
	})

	require.NoError(t, err)
	require.Empty(t, errs)

	// In dry-run mode no files should be written.
	entries, err := os.ReadDir(outputDir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestGenerate_NameMustMatch_Missing(t *testing.T) {
	layout := minimalLayout(t)
	layout.Config.NameMustMatch = "service"

	// values does NOT contain the "service" key.
	def := buildTemplateDef("my-service", map[string]cty.Value{
		"env": cty.StringVal("prod"),
	})

	outputDir := t.TempDir()
	errs, err := Generate(&Config{
		TemplateDef: def,
		Catalog:     layout,
		OutputDir:   outputDir,
	})

	require.NoError(t, err)
	require.NotEmpty(t, errs)
}

func TestGenerate_NameMustMatch_Mismatch(t *testing.T) {
	layout := minimalLayout(t)
	layout.Config.NameMustMatch = "service"

	// values has "service" key but its value doesn't match the template name.
	def := buildTemplateDef("my-service", map[string]cty.Value{
		"service": cty.StringVal("wrong-service"),
	})

	outputDir := t.TempDir()
	errs, err := Generate(&Config{
		TemplateDef: def,
		Catalog:     layout,
		OutputDir:   outputDir,
	})

	require.NoError(t, err)
	require.NotEmpty(t, errs)
}

func TestGenerate_WritePath(t *testing.T) {
	tmpDir := t.TempDir()

	// Build catalog structure.
	rootDir := filepath.Join(tmpDir, "root")
	require.NoError(t, os.MkdirAll(rootDir, 0o755))
	rootConfig := filepath.Join(rootDir, "terragrunt-root.hcl")
	require.NoError(t, os.WriteFile(rootConfig, []byte("# root config"), 0o644))

	projectDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	// Create a project-level terragrunt.hcl.
	projectTemplate := filepath.Join(projectDir, "terragrunt.hcl")
	require.NoError(t, os.WriteFile(projectTemplate, []byte(`include "root" {
  path = find_in_parent_folders("terragrunt-root.hcl")
}

inputs = {
  env = values.env
}
`), 0o644))

	// Create a service directory with a terragrunt.hcl template.
	cloudRunDir := filepath.Join(projectDir, "cloud-run")
	require.NoError(t, os.MkdirAll(cloudRunDir, 0o755))
	serviceTemplate := filepath.Join(cloudRunDir, "terragrunt.hcl")
	require.NoError(t, os.WriteFile(serviceTemplate, []byte(`include "root" {
  path = find_in_parent_folders("terragrunt-root.hcl")
}

inputs = {
  name = values.name
}
`), 0o644))

	// Build a catalog layout.
	layout := &catalog.Layout{
		RootConfig: rootConfig,
		ProjectDir: projectDir,
		Services: map[string]catalog.Service{
			"cloud-run": {
				Path:     "cloud-run",
				IsRegion: false,
			},
		},
		Config: &catalog.CatalogConfig{},
	}

	// Create a template definition.
	def := buildTemplateDef("acme-svc", map[string]cty.Value{
		"env":       cty.StringVal("staging"),
		"name":      cty.StringVal("acme-svc"),
		"cloud-run": cty.EmptyObjectVal, // Service is present in values
	})

	outputDir := t.TempDir()
	errs, err := Generate(&Config{
		TemplateDef: def,
		Catalog:     layout,
		OutputDir:   outputDir,
		DryRun:      false,
	})

	require.NoError(t, err)
	require.Empty(t, errs)

	// Verify output directory was created.
	stat, err := os.Stat(outputDir)
	require.NoError(t, err)
	assert.True(t, stat.IsDir())

	// Verify terragrunt-root.hcl was copied.
	rootDst := filepath.Join(outputDir, "terragrunt-root.hcl")
	rootContent, err := os.ReadFile(rootDst)
	require.NoError(t, err)
	assert.Equal(t, "# root config", string(rootContent))

	// Verify project template was written.
	projectDst := filepath.Join(outputDir, "acme-svc", "terragrunt.hcl")
	projectContent, err := os.ReadFile(projectDst)
	require.NoError(t, err)
	assert.Contains(t, string(projectContent), "staging")
	assert.NotContains(t, string(projectContent), "values.env")

	// Verify service template was written.
	serviceDst := filepath.Join(outputDir, "acme-svc", "cloud-run", "terragrunt.hcl")
	serviceContent, err := os.ReadFile(serviceDst)
	require.NoError(t, err)
	assert.Contains(t, string(serviceContent), "acme-svc")
	assert.NotContains(t, string(serviceContent), "values.name")
}

func TestCopyFile_Successful(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a source file.
	srcPath := filepath.Join(tmpDir, "source.txt")
	expectedContent := "test file content"
	require.NoError(t, os.WriteFile(srcPath, []byte(expectedContent), 0o644))

	// Copy it to a destination.
	dstPath := filepath.Join(tmpDir, "dest.txt")
	err := copyFile(srcPath, dstPath)

	require.NoError(t, err)

	// Verify the destination has the same content.
	dstContent, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, expectedContent, string(dstContent))
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	srcPath := filepath.Join(tmpDir, "nonexistent.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")

	err := copyFile(srcPath, dstPath)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading")
}
