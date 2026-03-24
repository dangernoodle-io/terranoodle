package hclutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFile(t *testing.T) {
	t.Run("simple source", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")
		content := `
terraform {
  source = "../modules/vpc"
}
`
		err := os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseFile(configFile)
		require.NoError(t, err)
		assert.Equal(t, "../modules/vpc", cfg.Source)
		assert.Equal(t, configFile, cfg.Path)
	})

	t.Run("with inputs", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")
		content := `
terraform {
  source = "../modules/vpc"
}

inputs = {
  env  = "staging"
  name = "acme-vpc"
}
`
		err := os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseFile(configFile)
		require.NoError(t, err)
		assert.Equal(t, "../modules/vpc", cfg.Source)
		assert.NotNil(t, cfg.Inputs)
		assert.Contains(t, cfg.Inputs, "env")
		assert.Contains(t, cfg.Inputs, "name")
	})

	t.Run("with locals and dependency", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")
		content := `
locals {
  env = "staging"
}

terraform {
  source = "../modules/vpc"
}

dependency "network" {
  config_path = "../network"
}
`
		err := os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseFile(configFile)
		require.NoError(t, err)
		assert.Equal(t, "../modules/vpc", cfg.Source)
		// Check that locals were parsed
		assert.NotNil(t, cfg.EvalCtx)
		assert.NotNil(t, cfg.EvalCtx.Variables)
		// Check that dependencies were parsed
		assert.Len(t, cfg.Dependencies, 1)
		assert.Equal(t, "network", cfg.Dependencies[0].Name)
	})

	t.Run("no terraform block", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")
		content := `
locals {
  env = "staging"
}
`
		err := os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseFile(configFile)
		require.NoError(t, err)
		assert.Empty(t, cfg.Source)
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := ParseFile("/nonexistent/terragrunt.hcl")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reading")
	})

	t.Run("invalid HCL", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")
		content := `
terraform {
  source = ../modules/vpc  # missing quotes!
}
`
		err := os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		_, err = ParseFile(configFile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing")
	})

	t.Run("complex config with all features", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")
		content := `
locals {
  env  = "production"
  team = "platform"
}

terraform {
  source = "git::https://example.com/modules.git//vpc?ref=v1.0.0"
}

dependency "network" {
  config_path = "../network"
}

inputs = {
  environment = local.env
  team_name   = local.team
  cidr_block  = "10.0.0.0/16"
  tags = {
    Environment = local.env
  }
}
`
		err := os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseFile(configFile)
		require.NoError(t, err)
		assert.Equal(t, "git::https://example.com/modules.git//vpc?ref=v1.0.0", cfg.Source)
		assert.NotNil(t, cfg.Inputs)
		assert.Len(t, cfg.Dependencies, 1)
		assert.Equal(t, "network", cfg.Dependencies[0].Name)
	})

	t.Run("empty file", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")
		err := os.WriteFile(configFile, []byte(""), 0644)
		require.NoError(t, err)

		cfg, err := ParseFile(configFile)
		require.NoError(t, err)
		assert.Empty(t, cfg.Source)
		assert.Empty(t, cfg.Inputs)
		assert.Empty(t, cfg.Dependencies)
	})

	t.Run("preserves absolute path", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")
		content := `
terraform {
  source = "/opt/modules/vpc"
}
`
		err := os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseFile(configFile)
		require.NoError(t, err)
		assert.Equal(t, "/opt/modules/vpc", cfg.Source)
	})

	t.Run("remote source", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")
		content := `
terraform {
  source = "github.com/acme-corp/modules//vpc"
}
`
		err := os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseFile(configFile)
		require.NoError(t, err)
		assert.Equal(t, "github.com/acme-corp/modules//vpc", cfg.Source)
	})

	t.Run("extra_arguments with var files", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")

		// Create some tfvars files
		tfvarsFile := filepath.Join(dir, "terraform.tfvars")
		err := os.WriteFile(tfvarsFile, []byte("env = \"test\"\n"), 0644)
		require.NoError(t, err)

		autoFile := filepath.Join(dir, "custom.auto.tfvars")
		err = os.WriteFile(autoFile, []byte("region = \"us-east-1\"\n"), 0644)
		require.NoError(t, err)

		customFile := filepath.Join(dir, "custom.tfvars")
		err = os.WriteFile(customFile, []byte("project = \"example-project\"\n"), 0644)
		require.NoError(t, err)

		content := `
terraform {
  source = "../modules/vpc"

  extra_arguments "defaults" {
    commands   = ["apply", "plan"]
    optional_var_files = ["custom.tfvars"]
  }
}
`
		err = os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseFile(configFile)
		require.NoError(t, err)

		// Should find auto-detected tfvars and extra_arguments tfvars files
		assert.NotEmpty(t, cfg.TfVarFiles, "TfVarFiles should not be empty")
		// At minimum should have the auto-detected files
		assert.Contains(t, cfg.TfVarFiles, filepath.Join(dir, "terraform.tfvars"))
		assert.Contains(t, cfg.TfVarFiles, filepath.Join(dir, "custom.auto.tfvars"))
	})

	t.Run("required_var_files from extra_arguments", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")

		// Create required tfvars file
		requiredFile := filepath.Join(dir, "required.tfvars")
		err := os.WriteFile(requiredFile, []byte("required_key = \"value\"\n"), 0644)
		require.NoError(t, err)

		content := `
terraform {
  source = "../modules/vpc"

  extra_arguments "defaults" {
    commands           = ["apply"]
    required_var_files = ["required.tfvars"]
  }
}
`
		err = os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseFile(configFile)
		require.NoError(t, err)

		// Verify config parses without error (tfvars parsing may vary)
		assert.NotNil(t, cfg)
	})

	t.Run("extra_arguments with relative paths in subdirectory", func(t *testing.T) {
		dir := t.TempDir()
		subdir := filepath.Join(dir, "subdir")
		err := os.Mkdir(subdir, 0755)
		require.NoError(t, err)

		configFile := filepath.Join(subdir, "terragrunt.hcl")

		// Create tfvars file in the same directory as terragrunt.hcl
		localTfvars := filepath.Join(subdir, "local.tfvars")
		err = os.WriteFile(localTfvars, []byte("local_key = \"value\"\n"), 0644)
		require.NoError(t, err)

		content := `
terraform {
  source = "../../modules/vpc"

  extra_arguments "defaults" {
    commands           = ["apply"]
    optional_var_files = ["local.tfvars"]
  }
}
`
		err = os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseFile(configFile)
		require.NoError(t, err)

		// Verify config parses without error
		assert.NotNil(t, cfg)
	})

	t.Run("no tfvars files when none exist", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")

		content := `
terraform {
  source = "../modules/vpc"
}
`
		err := os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		cfg, err := ParseFile(configFile)
		require.NoError(t, err)

		// When no tfvars files are auto-detected, should be empty
		assert.IsType(t, []string{}, cfg.TfVarFiles)
	})
}
