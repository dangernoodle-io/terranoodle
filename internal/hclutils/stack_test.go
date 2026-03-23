package hclutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStackFile(t *testing.T) {
	t.Run("simple unit", func(t *testing.T) {
		dir := t.TempDir()
		stackPath := filepath.Join(dir, "stack.hcl")
		content := `
unit "bootstrap" {
  source = "../modules/bootstrap"
}
`
		err := os.WriteFile(stackPath, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseStackFile(stackPath)
		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Len(t, cfg.Units, 1)
		assert.Equal(t, "bootstrap", cfg.Units[0].Name)
		assert.Equal(t, "../modules/bootstrap", cfg.Units[0].Source)
		assert.Equal(t, stackPath, cfg.Path)
	})

	t.Run("unit with values", func(t *testing.T) {
		dir := t.TempDir()
		stackPath := filepath.Join(dir, "stack.hcl")
		content := `
unit "vpc" {
  source = "../modules/vpc"
  values = {
    env  = "staging"
    name = "acme-vpc"
  }
}
`
		err := os.WriteFile(stackPath, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseStackFile(stackPath)
		require.NoError(t, err)
		require.Len(t, cfg.Units, 1)

		unit := cfg.Units[0]
		assert.Equal(t, "vpc", unit.Name)
		assert.Equal(t, "../modules/vpc", unit.Source)
		assert.NotNil(t, unit.Values)
		assert.Contains(t, unit.Values, "env")
		assert.Contains(t, unit.Values, "name")
	})

	t.Run("locals in stack", func(t *testing.T) {
		dir := t.TempDir()
		stackPath := filepath.Join(dir, "stack.hcl")
		content := `
locals {
  env = "staging"
}

unit "vpc" {
  source = "../modules/vpc"
}
`
		err := os.WriteFile(stackPath, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseStackFile(stackPath)
		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Len(t, cfg.Units, 1)
		assert.Equal(t, "vpc", cfg.Units[0].Name)

		// Verify that locals were processed in the eval context
		assert.NotNil(t, cfg.Units[0].EvalCtx)
		assert.NotNil(t, cfg.Units[0].EvalCtx.Variables)
	})

	t.Run("multiple units", func(t *testing.T) {
		dir := t.TempDir()
		stackPath := filepath.Join(dir, "stack.hcl")
		content := `
unit "bootstrap" {
  source = "../modules/bootstrap"
}

unit "vpc" {
  source = "../modules/vpc"
}

unit "database" {
  source = "../modules/database"
}
`
		err := os.WriteFile(stackPath, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseStackFile(stackPath)
		require.NoError(t, err)
		require.Len(t, cfg.Units, 3)

		names := make([]string, len(cfg.Units))
		for i, u := range cfg.Units {
			names[i] = u.Name
		}
		assert.Contains(t, names, "bootstrap")
		assert.Contains(t, names, "vpc")
		assert.Contains(t, names, "database")
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := ParseStackFile("/nonexistent/stack.hcl")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reading")
	})

	t.Run("invalid HCL", func(t *testing.T) {
		dir := t.TempDir()
		stackPath := filepath.Join(dir, "stack.hcl")
		content := `
unit "bootstrap" {
  source = ../modules/bootstrap  # missing quotes
}
`
		err := os.WriteFile(stackPath, []byte(content), 0644)
		require.NoError(t, err)

		_, err = ParseStackFile(stackPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing")
	})

	t.Run("empty file", func(t *testing.T) {
		dir := t.TempDir()
		stackPath := filepath.Join(dir, "stack.hcl")
		err := os.WriteFile(stackPath, []byte(""), 0644)
		require.NoError(t, err)

		cfg, err := ParseStackFile(stackPath)
		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Empty(t, cfg.Units)
	})

	t.Run("unit with complex values", func(t *testing.T) {
		dir := t.TempDir()
		stackPath := filepath.Join(dir, "stack.hcl")
		content := `
unit "app" {
  source = "../modules/app"
  values = {
    env      = "staging"
    replicas = 3
    enabled  = true
    tags = {
      team = "platform"
      env  = "staging"
    }
  }
}
`
		err := os.WriteFile(stackPath, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseStackFile(stackPath)
		require.NoError(t, err)
		require.Len(t, cfg.Units, 1)

		unit := cfg.Units[0]
		assert.Equal(t, "app", unit.Name)
		assert.NotNil(t, unit.Values)
		assert.Contains(t, unit.Values, "env")
		assert.Contains(t, unit.Values, "replicas")
		assert.Contains(t, unit.Values, "enabled")
		assert.Contains(t, unit.Values, "tags")
	})

	t.Run("unit without source is parsed", func(t *testing.T) {
		dir := t.TempDir()
		stackPath := filepath.Join(dir, "stack.hcl")
		content := `
unit "empty" {
  values = {
    name = "test"
  }
}
`
		err := os.WriteFile(stackPath, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseStackFile(stackPath)
		require.NoError(t, err)
		require.Len(t, cfg.Units, 1)
		assert.Equal(t, "empty", cfg.Units[0].Name)
		assert.Empty(t, cfg.Units[0].Source)
		assert.NotNil(t, cfg.Units[0].Values)
	})

	t.Run("locals with references", func(t *testing.T) {
		dir := t.TempDir()
		stackPath := filepath.Join(dir, "stack.hcl")
		content := `
locals {
  env     = "staging"
  project = "acme"
  name    = "${local.project}-${local.env}"
}

unit "app" {
  source = "../modules/app"
  values = {
    env = local.env
  }
}
`
		err := os.WriteFile(stackPath, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseStackFile(stackPath)
		require.NoError(t, err)
		require.Len(t, cfg.Units, 1)

		// Verify the eval context has locals
		assert.NotNil(t, cfg.Units[0].EvalCtx)
		assert.NotNil(t, cfg.Units[0].EvalCtx.Variables)
	})

	t.Run("multiple units with locals", func(t *testing.T) {
		dir := t.TempDir()
		stackPath := filepath.Join(dir, "stack.hcl")
		content := `
locals {
  env = "production"
}

unit "bootstrap" {
  source = "../modules/bootstrap"
  values = {
    environment = "prod"
  }
}

unit "vpc" {
  source = "../modules/vpc"
  values = {
    cidr = "10.0.0.0/16"
  }
}
`
		err := os.WriteFile(stackPath, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseStackFile(stackPath)
		require.NoError(t, err)
		require.Len(t, cfg.Units, 2)

		// Both units should share the same eval context with locals
		for _, unit := range cfg.Units {
			assert.NotNil(t, unit.EvalCtx)
			assert.NotNil(t, unit.EvalCtx.Variables)
		}
	})

	t.Run("unit with only source", func(t *testing.T) {
		dir := t.TempDir()
		stackPath := filepath.Join(dir, "stack.hcl")
		content := `
unit "minimal" {
  source = "../modules/minimal"
}
`
		err := os.WriteFile(stackPath, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseStackFile(stackPath)
		require.NoError(t, err)
		require.Len(t, cfg.Units, 1)

		unit := cfg.Units[0]
		assert.Equal(t, "minimal", unit.Name)
		assert.Equal(t, "../modules/minimal", unit.Source)
		assert.Nil(t, unit.Values)
	})
}
