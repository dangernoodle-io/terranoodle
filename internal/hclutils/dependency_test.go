package hclutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDependencyLabels(t *testing.T) {
	t.Run("no dependencies", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")
		content := `
terraform {
  source = "../modules/vpc"
}
`
		err := os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		labels, err := ParseDependencyLabels(configFile)
		require.NoError(t, err)
		assert.Empty(t, labels)
	})

	t.Run("single dependency", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")
		content := `
dependency "network" {
  config_path = "../network"
}
`
		err := os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		labels, err := ParseDependencyLabels(configFile)
		require.NoError(t, err)
		assert.Equal(t, []string{"network"}, labels)
	})

	t.Run("multiple dependencies", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")
		content := `
dependency "alpha" {
  config_path = "../alpha"
}

dependency "beta" {
  config_path = "../beta"
}

dependency "gamma" {
  config_path = "../gamma"
}
`
		err := os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		labels, err := ParseDependencyLabels(configFile)
		require.NoError(t, err)
		assert.Equal(t, []string{"alpha", "beta", "gamma"}, labels)
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := ParseDependencyLabels("/nonexistent/terragrunt.hcl")
		require.Error(t, err)
	})

	t.Run("invalid HCL", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")
		content := `
this is not valid hcl {
  syntax error !!
`
		err := os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		_, err = ParseDependencyLabels(configFile)
		require.Error(t, err)
	})

	t.Run("mixed blocks", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "terragrunt.hcl")
		content := `
terraform {
  source = "../modules/vpc"
}

locals {
  env = "staging"
}

dependency "network" {
  config_path = "../network"
}

inputs = {
  name = "acme-vpc"
}

dependency "security" {
  config_path = "../security"
}
`
		err := os.WriteFile(configFile, []byte(content), 0644)
		require.NoError(t, err)

		labels, err := ParseDependencyLabels(configFile)
		require.NoError(t, err)
		assert.Equal(t, []string{"network", "security"}, labels)
	})
}
