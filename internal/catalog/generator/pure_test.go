package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"dangernoodle.io/terratools/internal/catalog/catalog"
	"dangernoodle.io/terratools/internal/catalog/hclparse"
)

func TestSplitValues_AllProject(t *testing.T) {
	values := map[string]cty.Value{
		"env":     cty.StringVal("prod"),
		"region":  cty.StringVal("us-central1"),
		"project": cty.StringVal("acme-project"),
	}
	serviceBaseNames := map[string]bool{
		"redis":     true,
		"cloud-run": true,
	}

	project, services := splitValues(values, serviceBaseNames)

	assert.Len(t, project, 3)
	assert.Empty(t, services)
	assert.Contains(t, project, "env")
	assert.Contains(t, project, "region")
	assert.Contains(t, project, "project")
}

func TestSplitValues_AllServices(t *testing.T) {
	values := map[string]cty.Value{
		"redis":     cty.StringVal("redis-val"),
		"cloud-run": cty.StringVal("cloudrun-val"),
	}
	serviceBaseNames := map[string]bool{
		"redis":     true,
		"cloud-run": true,
	}

	project, services := splitValues(values, serviceBaseNames)

	assert.Empty(t, project)
	assert.Len(t, services, 2)
	assert.Contains(t, services, "redis")
	assert.Contains(t, services, "cloud-run")
}

func TestSplitValues_Mixed(t *testing.T) {
	values := map[string]cty.Value{
		"env":       cty.StringVal("prod"),
		"redis":     cty.StringVal("redis-val"),
		"region":    cty.StringVal("us-central1"),
		"cloud-run": cty.StringVal("cloudrun-val"),
	}
	serviceBaseNames := map[string]bool{
		"redis":     true,
		"cloud-run": true,
	}

	project, services := splitValues(values, serviceBaseNames)

	assert.Len(t, project, 2)
	assert.Len(t, services, 2)
	assert.Contains(t, project, "env")
	assert.Contains(t, project, "region")
	assert.Contains(t, services, "redis")
	assert.Contains(t, services, "cloud-run")
}

func TestMergeValues_ServiceOverridesProject(t *testing.T) {
	projectValues := map[string]cty.Value{
		"env":   cty.StringVal("prod"),
		"image": cty.StringVal("default-image"),
	}
	svcVals := cty.ObjectVal(map[string]cty.Value{
		"env": cty.StringVal("staging"),
	})

	merged := mergeValues(projectValues, svcVals)

	require.True(t, merged.Type().IsObjectType())
	env := merged.GetAttr("env")
	assert.Equal(t, "staging", env.AsString())
	image := merged.GetAttr("image")
	assert.Equal(t, "default-image", image.AsString())
}

func TestMergeValues_EmptyService(t *testing.T) {
	projectValues := map[string]cty.Value{
		"env": cty.StringVal("prod"),
	}
	svcVals := cty.EmptyObjectVal

	merged := mergeValues(projectValues, svcVals)

	require.True(t, merged.Type().IsObjectType())
	env := merged.GetAttr("env")
	assert.Equal(t, "prod", env.AsString())
}

func TestMergeIgnoreDeps_Union(t *testing.T) {
	catalogDeps := []string{"alpha"}
	templateDeps := []string{"beta"}

	result := mergeIgnoreDeps(catalogDeps, templateDeps)

	assert.Len(t, result, 2)
	assert.True(t, result["alpha"])
	assert.True(t, result["beta"])
}

func TestMergeIgnoreDeps_AllEmpty(t *testing.T) {
	result := mergeIgnoreDeps(nil, nil)

	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestMergeIgnoreDeps_Overlap(t *testing.T) {
	catalogDeps := []string{"alpha", "beta"}
	templateDeps := []string{"beta", "gamma"}

	result := mergeIgnoreDeps(catalogDeps, templateDeps)

	assert.Len(t, result, 3)
	assert.True(t, result["alpha"])
	assert.True(t, result["beta"])
	assert.True(t, result["gamma"])
}

func TestComputeRelPath_FlatService(t *testing.T) {
	relPath := computeRelPath("", "cloud-run", "parent-template", "redis")

	expected := "../../parent-template/redis"
	assert.Equal(t, expected, relPath)
}

func TestComputeRelPath_RegionalService(t *testing.T) {
	relPath := computeRelPath("", "region/redis", "parent-template", "cloud-run")

	expected := "../../../parent-template/cloud-run"
	assert.Equal(t, expected, relPath)
}

func TestCollapseBlankLines_TripleNewlines(t *testing.T) {
	input := []byte("line1\n\n\nline2")
	result := collapseBlankLines(input)

	expected := []byte("line1\n\nline2")
	assert.Equal(t, expected, result)
}

func TestCollapseBlankLines_QuadNewlines(t *testing.T) {
	input := []byte("line1\n\n\n\nline2")
	result := collapseBlankLines(input)

	expected := []byte("line1\n\nline2")
	assert.Equal(t, expected, result)
}

func TestCollapseBlankLines_DoubleNewlines(t *testing.T) {
	input := []byte("line1\n\nline2")
	result := collapseBlankLines(input)

	// Should not change
	assert.Equal(t, input, result)
}

func TestWarnUnusedServices_AllUsed(t *testing.T) {
	unitDefs := []hclparse.UnitDef{
		{
			Name:      "my-template",
			Values:    map[string]cty.Value{"redis": cty.StringVal("val")},
			RawValues: nil,
		},
	}

	serviceBaseNames := map[string]bool{
		"redis": true,
	}

	result := warnUnusedServices(unitDefs, serviceBaseNames)

	assert.Empty(t, result)
}

func TestWarnUnusedServices_SomeUnused(t *testing.T) {
	unitDefs := []hclparse.UnitDef{
		{
			Name:      "my-template",
			Values:    map[string]cty.Value{"redis": cty.StringVal("val")},
			RawValues: nil,
		},
	}

	serviceBaseNames := map[string]bool{
		"redis":     true,
		"cloud-run": true,
		"bigquery":  true,
	}

	result := warnUnusedServices(unitDefs, serviceBaseNames)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "cloud-run")
	assert.Contains(t, result, "bigquery")
}

func TestStripDependencyBlocks_RemovesLabeled(t *testing.T) {
	content := []byte(`
include "root" {
  path = find_in_parent_folders("terragrunt-root.hcl")
}

dependency "alpha" {
  config_path = "../alpha"
}

dependency "beta" {
  config_path = "../beta"
}
`)

	labels := map[string]bool{"alpha": true}
	result, err := StripDependencyBlocks(content, labels)

	require.NoError(t, err)
	assert.Contains(t, string(result), "beta")
	assert.NotContains(t, string(result), "alpha")
}

func TestStripDependencyBlocks_NothingToStrip(t *testing.T) {
	content := []byte(`
include "root" {
  path = find_in_parent_folders("terragrunt-root.hcl")
}

dependency "service" {
  config_path = "../service"
}
`)

	labels := map[string]bool{"other": true}
	result, err := StripDependencyBlocks(content, labels)

	require.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestStripDependencyBlocks_EmptyLabels(t *testing.T) {
	content := []byte(`
dependency "alpha" {
  config_path = "../alpha"
}
`)

	result, err := StripDependencyBlocks(content, nil)

	require.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestInjectDependencies_NoDeps(t *testing.T) {
	content := []byte(`include "root" {
  path = find_in_parent_folders("terragrunt-root.hcl")
}
`)

	dependsOn := make(map[string]map[string][]string)
	result, err := InjectDependencies(content, "my-template", dependsOn, "cloud-run")

	require.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestInjectDependencies_SingleDep(t *testing.T) {
	content := []byte(`include "root" {
  path = find_in_parent_folders("terragrunt-root.hcl")
}
`)

	dependsOn := map[string]map[string][]string{
		"parent-template": {
			"redis": {"cloud-run"},
		},
	}

	result, err := InjectDependencies(content, "my-template", dependsOn, "cloud-run")

	require.NoError(t, err)
	assert.Contains(t, string(result), "dependencies {")
	assert.Contains(t, string(result), "dependency \"redis\"")
}

func TestValidateServiceDeps_AllSatisfied(t *testing.T) {
	values := map[string]cty.Value{
		"env":   cty.StringVal("prod"),
		"redis": cty.StringVal("redis-val"),
	}

	layout := &catalog.Layout{
		Services: map[string]catalog.Service{
			"redis": {
				Path:         "redis",
				IsRegion:     false,
				Dependencies: []string{},
			},
		},
		Config: &catalog.CatalogConfig{},
	}

	ignoreDeps := make(map[string]bool)
	errs := validateServiceDeps("my-template", values, layout, ignoreDeps)

	assert.Empty(t, errs)
}

func TestValidateServiceDeps_MissingDep(t *testing.T) {
	values := map[string]cty.Value{
		"cloud-run": cty.StringVal("cloudrun-val"),
	}

	layout := &catalog.Layout{
		Services: map[string]catalog.Service{
			"cloud-run": {
				Path:         "cloud-run",
				IsRegion:     false,
				Dependencies: []string{"redis"},
			},
			"redis": {
				Path:         "redis",
				IsRegion:     false,
				Dependencies: []string{},
			},
		},
		Config: &catalog.CatalogConfig{},
	}

	ignoreDeps := make(map[string]bool)
	errs := validateServiceDeps("my-template", values, layout, ignoreDeps)

	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Detail, "redis")
}

func TestValidateServiceDeps_IgnoredDep(t *testing.T) {
	values := map[string]cty.Value{
		"cloud-run": cty.StringVal("cloudrun-val"),
	}

	layout := &catalog.Layout{
		Services: map[string]catalog.Service{
			"cloud-run": {
				Path:         "cloud-run",
				IsRegion:     false,
				Dependencies: []string{"redis"},
			},
			"redis": {
				Path:         "redis",
				IsRegion:     false,
				Dependencies: []string{},
			},
		},
		Config: &catalog.CatalogConfig{},
	}

	ignoreDeps := map[string]bool{"redis": true}
	errs := validateServiceDeps("my-template", values, layout, ignoreDeps)

	assert.Empty(t, errs)
}

func TestResolveCrossTemplateDeps_IntraTemplate(t *testing.T) {
	templates := []struct {
		Name     string
		Services map[string]bool
	}{
		{
			Name: "my-template",
			Services: map[string]bool{
				"cloud-run": true,
				"redis":     true,
			},
		},
	}

	layout := &catalog.Layout{
		Services: map[string]catalog.Service{
			"cloud-run": {
				Path:         "cloud-run",
				IsRegion:     false,
				Dependencies: []string{"redis"},
			},
			"redis": {
				Path:         "redis",
				IsRegion:     false,
				Dependencies: []string{},
			},
		},
		Config: &catalog.CatalogConfig{},
	}

	ignoreDeps := make(map[string]bool)
	crossDeps, err := ResolveCrossTemplateDeps(templates, layout, ignoreDeps)

	require.NoError(t, err)
	assert.Empty(t, crossDeps)
}

func TestResolveCrossTemplateDeps_CrossTemplate(t *testing.T) {
	templates := []struct {
		Name     string
		Services map[string]bool
	}{
		{
			Name: "template-a",
			Services: map[string]bool{
				"cloud-run": true,
			},
		},
		{
			Name: "template-b",
			Services: map[string]bool{
				"redis": true,
			},
		},
	}

	layout := &catalog.Layout{
		Services: map[string]catalog.Service{
			"cloud-run": {
				Path:         "cloud-run",
				IsRegion:     false,
				Dependencies: []string{"redis"},
			},
			"redis": {
				Path:         "redis",
				IsRegion:     false,
				Dependencies: []string{},
			},
		},
		Config: &catalog.CatalogConfig{},
	}

	ignoreDeps := make(map[string]bool)
	crossDeps, err := ResolveCrossTemplateDeps(templates, layout, ignoreDeps)

	require.NoError(t, err)
	require.Len(t, crossDeps, 1)
	assert.Equal(t, "template-a", crossDeps[0].SourceTemplate)
	assert.Equal(t, "cloud-run", crossDeps[0].SourceService)
	assert.Equal(t, "template-b", crossDeps[0].TargetTemplate)
	assert.Equal(t, "redis", crossDeps[0].TargetService)
}

func TestResolveCrossTemplateDeps_AmbiguousDep(t *testing.T) {
	templates := []struct {
		Name     string
		Services map[string]bool
	}{
		{
			Name: "template-a",
			Services: map[string]bool{
				"cloud-run": true,
			},
		},
		{
			Name: "template-b",
			Services: map[string]bool{
				"redis": true,
			},
		},
		{
			Name: "template-c",
			Services: map[string]bool{
				"redis": true,
			},
		},
	}

	layout := &catalog.Layout{
		Services: map[string]catalog.Service{
			"cloud-run": {
				Path:         "cloud-run",
				IsRegion:     false,
				Dependencies: []string{"redis"},
			},
			"redis": {
				Path:         "redis",
				IsRegion:     false,
				Dependencies: []string{},
			},
		},
		Config: &catalog.CatalogConfig{},
	}

	ignoreDeps := make(map[string]bool)
	_, err := ResolveCrossTemplateDeps(templates, layout, ignoreDeps)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
}
