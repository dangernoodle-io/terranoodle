package generator

import (
	"os"
	"path/filepath"
	"strings"
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

func TestGenerate_NameMustMatch_NonString(t *testing.T) {
	layout := minimalLayout(t)
	layout.Config.NameMustMatch = "service"

	// values has "service" key but its value is a boolean instead of a string.
	def := buildTemplateDef("my-service", map[string]cty.Value{
		"service": cty.BoolVal(true),
	})

	outputDir := t.TempDir()
	errs, err := Generate(&Config{
		TemplateDef: def,
		Catalog:     layout,
		OutputDir:   outputDir,
	})

	require.NoError(t, err)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Detail, "must be a string")
}

func TestValidateIgnoreDeps_ValidEntry(t *testing.T) {
	// Create a layout with a service that has dependencies.
	layout := minimalLayout(t)
	layout.Services = map[string]catalog.Service{
		"api-gateway": {
			Path:         "api-gateway",
			IsRegion:     false,
			Dependencies: []string{"postgres", "redis"},
		},
		"postgres": {
			Path:         "postgres",
			IsRegion:     false,
			Dependencies: []string{},
		},
		"redis": {
			Path:         "redis",
			IsRegion:     false,
			Dependencies: []string{},
		},
	}

	// Test with a valid ignoreDeps entry that matches an actual dependency.
	ignoreDeps := map[string]bool{"postgres": true}
	errs := validateIgnoreDeps(ignoreDeps, layout)

	require.Empty(t, errs)
}

func TestValidateIgnoreDeps_InvalidEntry(t *testing.T) {
	// Create a layout with a service that has dependencies.
	layout := minimalLayout(t)
	layout.Services = map[string]catalog.Service{
		"api-gateway": {
			Path:         "api-gateway",
			IsRegion:     false,
			Dependencies: []string{"postgres", "redis"},
		},
		"postgres": {
			Path:         "postgres",
			IsRegion:     false,
			Dependencies: []string{},
		},
		"redis": {
			Path:         "redis",
			IsRegion:     false,
			Dependencies: []string{},
		},
	}

	// Test with an invalid ignoreDeps entry that doesn't match any dependency.
	// This simulates a typo in ignore_deps configuration.
	ignoreDeps := map[string]bool{"postgre": true} // Missing 's' - a common typo
	errs := validateIgnoreDeps(ignoreDeps, layout)

	require.NotEmpty(t, errs)
	require.Equal(t, 1, len(errs))
	assert.Contains(t, errs[0].Detail, "postgre")
	assert.Contains(t, errs[0].Detail, "does not match")
}

func TestValidateIgnoreDeps_MultipleInvalidEntries(t *testing.T) {
	layout := minimalLayout(t)
	layout.Services = map[string]catalog.Service{
		"api-gateway": {
			Path:         "api-gateway",
			IsRegion:     false,
			Dependencies: []string{"postgres"},
		},
	}

	// Test with multiple invalid entries.
	ignoreDeps := map[string]bool{
		"postgre":  true,
		"nonexist": true,
	}
	errs := validateIgnoreDeps(ignoreDeps, layout)

	require.Equal(t, 2, len(errs))
}

func TestBroadenedValuesKeyValidation_TypoDetection(t *testing.T) {
	layout := minimalLayout(t)
	layout.Services = map[string]catalog.Service{
		"redis": {
			Path:     "redis",
			IsRegion: false,
		},
		"api-gateway": {
			Path:     "api-gateway",
			IsRegion: false,
		},
	}

	// Create a template with a misspelled service name (edit distance 1 from "redis").
	def := buildTemplateDef("acme-svc", map[string]cty.Value{
		"rediss": cty.EmptyObjectVal, // Typo: extra 's'
		"env":    cty.StringVal("prod"),
	})

	outputDir := t.TempDir()
	errs, err := Generate(&Config{
		TemplateDef: def,
		Catalog:     layout,
		OutputDir:   outputDir,
	})

	require.NoError(t, err)
	require.NotEmpty(t, errs)
	// Should have a warning about "rediss" with suggestion for "redis".
	assert.True(t, len(errs) > 0, "expected at least one error")
	foundWarning := false
	for _, e := range errs {
		if strings.Contains(e.Detail, "rediss") && strings.Contains(e.Detail, "did you mean") {
			foundWarning = true
			assert.Contains(t, e.Detail, "redis")
			break
		}
	}
	assert.True(t, foundWarning, "expected warning about 'rediss' with suggestion")
}

func TestBroadenedValuesKeyValidation_NoFalsePositives(t *testing.T) {
	tmpDir := t.TempDir()

	// Build minimal catalog structure.
	rootDir := filepath.Join(tmpDir, "root")
	require.NoError(t, os.MkdirAll(rootDir, 0o755))
	rootConfig := filepath.Join(rootDir, "terragrunt-root.hcl")
	require.NoError(t, os.WriteFile(rootConfig, []byte("# root config"), 0o644))

	projectDir := filepath.Join(tmpDir, "project")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	// Create a project-level terragrunt.hcl template (required).
	projectTemplate := filepath.Join(projectDir, "terragrunt.hcl")
	require.NoError(t, os.WriteFile(projectTemplate, []byte(`include "root" {
  path = find_in_parent_folders("terragrunt-root.hcl")
}

inputs = {
  tenant_id = values.tenant_id
}
`), 0o644))

	// Create redis service directory with template.
	redisDir := filepath.Join(projectDir, "redis")
	require.NoError(t, os.MkdirAll(redisDir, 0o755))
	redisTemplate := filepath.Join(redisDir, "terragrunt.hcl")
	require.NoError(t, os.WriteFile(redisTemplate, []byte(`include "root" {
  path = find_in_parent_folders("terragrunt-root.hcl")
}

inputs = {}
`), 0o644))

	layout := &catalog.Layout{
		RootConfig: rootConfig,
		ProjectDir: projectDir,
		Services: map[string]catalog.Service{
			"redis": {
				Path:     "redis",
				IsRegion: false,
			},
		},
		Config: &catalog.CatalogConfig{},
	}

	// Create a template with legitimate project-level values that are
	// not similar to any service name.
	def := buildTemplateDef("acme-svc", map[string]cty.Value{
		"tenant_id":   cty.StringVal("tenant-123"),
		"environment": cty.StringVal("prod"),
		"redis":       cty.EmptyObjectVal, // Legitimate service reference
	})

	outputDir := t.TempDir()
	errs, err := Generate(&Config{
		TemplateDef: def,
		Catalog:     layout,
		OutputDir:   outputDir,
	})

	require.NoError(t, err)
	// Should not have warnings about "tenant_id" or "environment" since they
	// are not similar to any service name.
	for _, e := range errs {
		assert.False(t, strings.Contains(e.Detail, "tenant_id"), "should not warn about legitimate project value")
		assert.False(t, strings.Contains(e.Detail, "environment"), "should not warn about legitimate project value")
	}
}

func TestSimilarServiceName_CaseInsensitivematch(t *testing.T) {
	serviceBaseNames := map[string]bool{
		"redis":       true,
		"api-gateway": true,
	}

	// Test case-insensitive match.
	match := similarServiceName("Redis", serviceBaseNames)
	assert.Equal(t, "redis", match)
}

func TestSimilarServiceName_LevenshteinMatch(t *testing.T) {
	serviceBaseNames := map[string]bool{
		"redis":       true,
		"api-gateway": true,
	}

	// Test Levenshtein distance 1 (one character addition).
	match := similarServiceName("rediss", serviceBaseNames)
	assert.Equal(t, "redis", match)

	// Test Levenshtein distance 2 (two character difference).
	match = similarServiceName("redisi", serviceBaseNames)
	assert.Equal(t, "redis", match)
}

func TestSimilarServiceName_NoMatch(t *testing.T) {
	serviceBaseNames := map[string]bool{
		"redis":       true,
		"api-gateway": true,
	}

	// Test that completely dissimilar names don't match.
	match := similarServiceName("tenant_id", serviceBaseNames)
	assert.Equal(t, "", match)

	// Test Levenshtein distance > 2 (should not match).
	match = similarServiceName("redissss", serviceBaseNames) // 3 extra characters
	assert.Equal(t, "", match)
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "b", 1},
		{"abc", "abc", 0},
		{"abc", "abd", 1},                // substitution
		{"abc", "ab", 1},                 // deletion
		{"abc", "abcd", 1},               // insertion
		{"kitten", "sitting", 3},         // multiple edits
		{"redis", "rediss", 1},           // extra 's'
		{"api-gateway", "api-gatewa", 1}, // missing last char
		{"postgres", "postgre", 1},       // missing 's'
	}

	for _, tt := range tests {
		t.Run(tt.a+"-"+tt.b, func(t *testing.T) {
			dist := levenshtein(tt.a, tt.b)
			assert.Equal(t, tt.expected, dist, "levenshtein(%q, %q)", tt.a, tt.b)
		})
	}
}
