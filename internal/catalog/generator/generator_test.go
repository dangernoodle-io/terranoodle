package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zclconf/go-cty/cty"

	"dangernoodle.io/terra-tools/internal/catalog/catalog"
	"dangernoodle.io/terra-tools/internal/catalog/hclparse"
)

// minimalLayout returns a catalog Layout with no services suitable for
// tests that only exercise template-level validation.
func minimalLayout(t *testing.T) *catalog.Layout {
	t.Helper()
	dir := t.TempDir()

	// Create the minimum directory structure required for a non-nil Layout.
	rootDir := filepath.Join(dir, "root")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatalf("creating root dir: %v", err)
	}
	rootConfig := filepath.Join(rootDir, "terragrunt-root.hcl")
	if err := os.WriteFile(rootConfig, []byte(`# root config`), 0o644); err != nil {
		t.Fatalf("creating root config: %v", err)
	}
	projectDir := filepath.Join(dir, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("creating project dir: %v", err)
	}

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

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}

	// In dry-run mode no files should be written.
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("reading output dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files written during dry-run, found %d entries", len(entries))
	}
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

	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(errs) == 0 {
		t.Fatal("expected validation errors for missing name_must_match key, got none")
	}
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

	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if len(errs) == 0 {
		t.Fatal("expected validation error for name_must_match mismatch, got none")
	}
}
