package hclutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestResolveIncludeLocals(t *testing.T) {
	t.Run("file with locals", func(t *testing.T) {
		dir := t.TempDir()
		includePath := filepath.Join(dir, "include.hcl")
		content := `
locals {
  env     = "staging"
  project = "acme"
}
`
		err := os.WriteFile(includePath, []byte(content), 0644)
		require.NoError(t, err)

		result, err := ResolveIncludeLocals(includePath)
		require.NoError(t, err)
		assert.True(t, result.Type().IsObjectType())
		assert.True(t, result.IsKnown())

		// Check for "env" and "project" attributes
		envVal := result.GetAttr("env")
		assert.Equal(t, cty.StringVal("staging"), envVal)

		projectVal := result.GetAttr("project")
		assert.Equal(t, cty.StringVal("acme"), projectVal)
	})

	t.Run("file without locals", func(t *testing.T) {
		dir := t.TempDir()
		includePath := filepath.Join(dir, "include.hcl")
		content := `
terraform {
  source = "../modules"
}
`
		err := os.WriteFile(includePath, []byte(content), 0644)
		require.NoError(t, err)

		result, err := ResolveIncludeLocals(includePath)
		require.NoError(t, err)
		assert.Equal(t, cty.EmptyObjectVal, result)
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := ResolveIncludeLocals("/nonexistent/file.hcl")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reading")
	})

	t.Run("invalid HCL", func(t *testing.T) {
		dir := t.TempDir()
		includePath := filepath.Join(dir, "include.hcl")
		content := `
locals {
  env = "staging
}
`
		err := os.WriteFile(includePath, []byte(content), 0644)
		require.NoError(t, err)

		_, err = ResolveIncludeLocals(includePath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing")
	})

	t.Run("empty file", func(t *testing.T) {
		dir := t.TempDir()
		includePath := filepath.Join(dir, "include.hcl")
		err := os.WriteFile(includePath, []byte(""), 0644)
		require.NoError(t, err)

		result, err := ResolveIncludeLocals(includePath)
		require.NoError(t, err)
		assert.Equal(t, cty.EmptyObjectVal, result)
	})

	t.Run("locals with various types", func(t *testing.T) {
		dir := t.TempDir()
		includePath := filepath.Join(dir, "include.hcl")
		content := `
locals {
  string_val = "test"
  number_val = 42
  bool_val   = true
}
`
		err := os.WriteFile(includePath, []byte(content), 0644)
		require.NoError(t, err)

		result, err := ResolveIncludeLocals(includePath)
		require.NoError(t, err)
		assert.True(t, result.Type().IsObjectType())

		assert.Equal(t, cty.StringVal("test"), result.GetAttr("string_val"))
		assert.Equal(t, cty.Number, result.GetAttr("number_val").Type())
		assert.Equal(t, cty.BoolVal(true), result.GetAttr("bool_val"))
	})

	t.Run("relative path resolution", func(t *testing.T) {
		dir := t.TempDir()
		includePath := filepath.Join(dir, "include.hcl")
		content := `
locals {
  relative_test = "works"
}
`
		err := os.WriteFile(includePath, []byte(content), 0644)
		require.NoError(t, err)

		// Use relative path; function should resolve to absolute
		t.Chdir(dir)

		result, err := ResolveIncludeLocals("include.hcl")
		require.NoError(t, err)
		assert.Equal(t, cty.StringVal("works"), result.GetAttr("relative_test"))
	})
}

func TestBuildIncludeVariable(t *testing.T) {
	t.Run("no exposed includes", func(t *testing.T) {
		includes := []IncludeConfig{
			{Name: "root", Path: "/some/path", Expose: false},
		}

		result := BuildIncludeVariable(includes)
		assert.Equal(t, cty.EmptyObjectVal, result)
	})

	t.Run("one exposed include with locals", func(t *testing.T) {
		dir := t.TempDir()
		includePath := filepath.Join(dir, "include.hcl")
		content := `
locals {
  env     = "staging"
  project = "acme"
}
`
		err := os.WriteFile(includePath, []byte(content), 0644)
		require.NoError(t, err)

		includes := []IncludeConfig{
			{Name: "root", Path: includePath, Expose: true},
		}

		result := BuildIncludeVariable(includes)
		require.True(t, result.Type().IsObjectType())
		require.True(t, result.IsKnown())

		// Check root.locals exists
		rootVal := result.GetAttr("root")
		assert.True(t, rootVal.Type().IsObjectType())

		localsVal := rootVal.GetAttr("locals")
		assert.True(t, localsVal.Type().IsObjectType())

		// Verify locals contents
		assert.Equal(t, cty.StringVal("staging"), localsVal.GetAttr("env"))
		assert.Equal(t, cty.StringVal("acme"), localsVal.GetAttr("project"))
	})

	t.Run("multiple exposed includes", func(t *testing.T) {
		dir := t.TempDir()

		// Create first include
		includePath1 := filepath.Join(dir, "include1.hcl")
		content1 := `
locals {
  name1 = "first"
}
`
		err := os.WriteFile(includePath1, []byte(content1), 0644)
		require.NoError(t, err)

		// Create second include
		includePath2 := filepath.Join(dir, "include2.hcl")
		content2 := `
locals {
  name2 = "second"
}
`
		err = os.WriteFile(includePath2, []byte(content2), 0644)
		require.NoError(t, err)

		includes := []IncludeConfig{
			{Name: "alpha", Path: includePath1, Expose: true},
			{Name: "beta", Path: includePath2, Expose: true},
		}

		result := BuildIncludeVariable(includes)
		require.True(t, result.Type().IsObjectType())

		// Check both includes exist
		alphaVal := result.GetAttr("alpha")
		assert.True(t, alphaVal.Type().IsObjectType())

		betaVal := result.GetAttr("beta")
		assert.True(t, betaVal.Type().IsObjectType())
	})

	t.Run("empty path is skipped", func(t *testing.T) {
		includes := []IncludeConfig{
			{Name: "root", Path: "", Expose: true},
		}

		result := BuildIncludeVariable(includes)
		assert.Equal(t, cty.EmptyObjectVal, result)
	})

	t.Run("mixed exposed and not exposed", func(t *testing.T) {
		dir := t.TempDir()

		// Create first include
		includePath1 := filepath.Join(dir, "include1.hcl")
		content1 := `
locals {
  env = "staging"
}
`
		err := os.WriteFile(includePath1, []byte(content1), 0644)
		require.NoError(t, err)

		// Create second include
		includePath2 := filepath.Join(dir, "include2.hcl")
		content2 := `
locals {
  other = "value"
}
`
		err = os.WriteFile(includePath2, []byte(content2), 0644)
		require.NoError(t, err)

		includes := []IncludeConfig{
			{Name: "alpha", Path: includePath1, Expose: true},
			{Name: "beta", Path: includePath2, Expose: false},
		}

		result := BuildIncludeVariable(includes)
		require.True(t, result.Type().IsObjectType())

		// Only alpha should be present
		alphaVal := result.GetAttr("alpha")
		assert.True(t, alphaVal.Type().IsObjectType())

		// Beta should not be present (not exposed)
		// Check that beta key doesn't exist
		assert.NotContains(t, result.AsValueMap(), "beta")
	})

	t.Run("exposed include with missing file is skipped", func(t *testing.T) {
		includes := []IncludeConfig{
			{Name: "root", Path: "/nonexistent/path", Expose: true},
		}

		result := BuildIncludeVariable(includes)
		// Function skips includes with errors, so result should be empty
		assert.Equal(t, cty.EmptyObjectVal, result)
	})

	t.Run("exposed include without locals", func(t *testing.T) {
		dir := t.TempDir()
		includePath := filepath.Join(dir, "include.hcl")
		content := `
terraform {
  source = "../modules"
}
`
		err := os.WriteFile(includePath, []byte(content), 0644)
		require.NoError(t, err)

		includes := []IncludeConfig{
			{Name: "root", Path: includePath, Expose: true},
		}

		result := BuildIncludeVariable(includes)
		require.True(t, result.Type().IsObjectType())

		// root should exist with empty locals
		rootVal := result.GetAttr("root")
		assert.True(t, rootVal.Type().IsObjectType())

		localsVal := rootVal.GetAttr("locals")
		assert.Equal(t, cty.EmptyObjectVal, localsVal)
	})
}
