package hclutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseModuleCalls(t *testing.T) {
	t.Run("empty dir", func(t *testing.T) {
		dir := t.TempDir()

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		assert.Nil(t, calls)
	})

	t.Run("module with source and inputs", func(t *testing.T) {
		dir := t.TempDir()
		mainPath := filepath.Join(dir, "main.tf")
		content := `
module "vpc" {
  source = "../modules/vpc"
  env    = "staging"
  name   = "acme-vpc"
}
`
		err := os.WriteFile(mainPath, []byte(content), 0644)
		require.NoError(t, err)

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		require.Len(t, calls, 1)

		module := calls[0]
		assert.Equal(t, "vpc", module.Name)
		assert.Equal(t, "../modules/vpc", module.Source)
		assert.NotNil(t, module.Inputs)
		assert.Contains(t, module.Inputs, "env")
		assert.Contains(t, module.Inputs, "name")
	})

	t.Run("no source attribute is skipped", func(t *testing.T) {
		dir := t.TempDir()
		mainPath := filepath.Join(dir, "main.tf")
		content := `
module "incomplete" {
  env = "staging"
  name = "acme"
}
`
		err := os.WriteFile(mainPath, []byte(content), 0644)
		require.NoError(t, err)

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		assert.Nil(t, calls)
	})

	t.Run("variable defaults in context", func(t *testing.T) {
		dir := t.TempDir()

		// Create variables.tf
		varsPath := filepath.Join(dir, "variables.tf")
		varsContent := `
variable "env" {
  default = "staging"
}

variable "region" {
  default = "us-east-1"
}
`
		err := os.WriteFile(varsPath, []byte(varsContent), 0644)
		require.NoError(t, err)

		// Create main.tf that uses the variables
		mainPath := filepath.Join(dir, "main.tf")
		mainContent := `
module "vpc" {
  source = "../modules/vpc"
  env    = var.env
  region = var.region
}
`
		err = os.WriteFile(mainPath, []byte(mainContent), 0644)
		require.NoError(t, err)

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		require.Len(t, calls, 1)

		module := calls[0]
		assert.Equal(t, "vpc", module.Name)
		assert.Equal(t, "../modules/vpc", module.Source)
		assert.NotNil(t, module.Inputs)
		assert.Contains(t, module.Inputs, "env")
		assert.Contains(t, module.Inputs, "region")
	})

	t.Run("multiple modules in one file", func(t *testing.T) {
		dir := t.TempDir()
		mainPath := filepath.Join(dir, "main.tf")
		content := `
module "vpc" {
  source = "../modules/vpc"
  name   = "acme-vpc"
}

module "database" {
  source = "../modules/database"
  name   = "acme-db"
}
`
		err := os.WriteFile(mainPath, []byte(content), 0644)
		require.NoError(t, err)

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		require.Len(t, calls, 2)

		names := make([]string, len(calls))
		for i, c := range calls {
			names[i] = c.Name
		}
		assert.Contains(t, names, "vpc")
		assert.Contains(t, names, "database")
	})

	t.Run("multiple modules across files", func(t *testing.T) {
		dir := t.TempDir()

		// Create main.tf
		mainPath := filepath.Join(dir, "main.tf")
		mainContent := `
module "vpc" {
  source = "../modules/vpc"
}
`
		err := os.WriteFile(mainPath, []byte(mainContent), 0644)
		require.NoError(t, err)

		// Create network.tf
		networkPath := filepath.Join(dir, "network.tf")
		networkContent := `
module "subnet" {
  source = "../modules/subnet"
}
`
		err = os.WriteFile(networkPath, []byte(networkContent), 0644)
		require.NoError(t, err)

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		require.Len(t, calls, 2)

		names := make([]string, len(calls))
		for i, c := range calls {
			names[i] = c.Name
		}
		assert.Contains(t, names, "vpc")
		assert.Contains(t, names, "subnet")
	})

	t.Run("nonexistent dir", func(t *testing.T) {
		_, err := ParseModuleCalls("/nonexistent/dir")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reading dir")
	})

	t.Run("module with meta arguments", func(t *testing.T) {
		dir := t.TempDir()
		mainPath := filepath.Join(dir, "main.tf")
		content := `
module "vpc" {
  source = "../modules/vpc"
  version = "~> 1.0"

  env  = "staging"
  name = "acme-vpc"

  count = 1
}
`
		err := os.WriteFile(mainPath, []byte(content), 0644)
		require.NoError(t, err)

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		require.Len(t, calls, 1)

		module := calls[0]
		assert.Equal(t, "vpc", module.Name)
		assert.Equal(t, "../modules/vpc", module.Source)
		// Meta arguments (version, count) should not be in inputs
		assert.Contains(t, module.Inputs, "env")
		assert.Contains(t, module.Inputs, "name")
		assert.NotContains(t, module.Inputs, "version")
		assert.NotContains(t, module.Inputs, "count")
	})

	t.Run("module with for_each", func(t *testing.T) {
		dir := t.TempDir()
		mainPath := filepath.Join(dir, "main.tf")
		content := `
module "instances" {
  source = "../modules/instance"

  for_each = {
    "app1" = "value1"
    "app2" = "value2"
  }

  name = each.value
}
`
		err := os.WriteFile(mainPath, []byte(content), 0644)
		require.NoError(t, err)

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		require.Len(t, calls, 1)

		module := calls[0]
		assert.Equal(t, "instances", module.Name)
		assert.NotContains(t, module.Inputs, "for_each")
	})

	t.Run("locals in terraform files", func(t *testing.T) {
		dir := t.TempDir()

		// Create locals.tf
		localsPath := filepath.Join(dir, "locals.tf")
		localsContent := `
locals {
  env = "staging"
}
`
		err := os.WriteFile(localsPath, []byte(localsContent), 0644)
		require.NoError(t, err)

		// Create main.tf that uses locals
		mainPath := filepath.Join(dir, "main.tf")
		mainContent := `
module "vpc" {
  source = "../modules/vpc"
  env    = local.env
}
`
		err = os.WriteFile(mainPath, []byte(mainContent), 0644)
		require.NoError(t, err)

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		require.Len(t, calls, 1)

		module := calls[0]
		assert.Equal(t, "vpc", module.Name)
		assert.Contains(t, module.Inputs, "env")
	})

	t.Run("remote source", func(t *testing.T) {
		dir := t.TempDir()
		mainPath := filepath.Join(dir, "main.tf")
		content := `
module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
  version = "~> 2.0"

  name = "acme-vpc"
}
`
		err := os.WriteFile(mainPath, []byte(content), 0644)
		require.NoError(t, err)

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		require.Len(t, calls, 1)

		module := calls[0]
		assert.Equal(t, "vpc", module.Name)
		assert.Equal(t, "terraform-aws-modules/vpc/aws", module.Source)
	})

	t.Run("git source", func(t *testing.T) {
		dir := t.TempDir()
		mainPath := filepath.Join(dir, "main.tf")
		content := `
module "vpc" {
  source = "git::https://github.com/acme/modules.git//vpc?ref=v1.0.0"

  name = "acme-vpc"
}
`
		err := os.WriteFile(mainPath, []byte(content), 0644)
		require.NoError(t, err)

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		require.Len(t, calls, 1)

		module := calls[0]
		assert.Equal(t, "vpc", module.Name)
		assert.Equal(t, "git::https://github.com/acme/modules.git//vpc?ref=v1.0.0", module.Source)
	})

	t.Run("ignore non-tf files", func(t *testing.T) {
		dir := t.TempDir()

		// Create a .tf file
		mainPath := filepath.Join(dir, "main.tf")
		mainContent := `
module "vpc" {
  source = "../modules/vpc"
}
`
		err := os.WriteFile(mainPath, []byte(mainContent), 0644)
		require.NoError(t, err)

		// Create a non-.tf file
		otherPath := filepath.Join(dir, "readme.txt")
		err = os.WriteFile(otherPath, []byte("some content"), 0644)
		require.NoError(t, err)

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		require.Len(t, calls, 1)
		assert.Equal(t, "vpc", calls[0].Name)
	})

	t.Run("multiple variables and locals", func(t *testing.T) {
		dir := t.TempDir()

		// Create variables.tf with multiple variables
		varsPath := filepath.Join(dir, "variables.tf")
		varsContent := `
variable "env" {
  default = "staging"
}

variable "region" {
  default = "us-east-1"
}

variable "tags" {
  default = {
    team = "platform"
  }
}
`
		err := os.WriteFile(varsPath, []byte(varsContent), 0644)
		require.NoError(t, err)

		// Create locals.tf
		localsPath := filepath.Join(dir, "locals.tf")
		localsContent := `
locals {
  name_prefix = "acme"
}
`
		err = os.WriteFile(localsPath, []byte(localsContent), 0644)
		require.NoError(t, err)

		// Create main.tf
		mainPath := filepath.Join(dir, "main.tf")
		mainContent := `
module "vpc" {
  source = "../modules/vpc"
  env    = var.env
  region = var.region
  name   = local.name_prefix
}
`
		err = os.WriteFile(mainPath, []byte(mainContent), 0644)
		require.NoError(t, err)

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		require.Len(t, calls, 1)

		module := calls[0]
		assert.Equal(t, "vpc", module.Name)
		assert.Contains(t, module.Inputs, "env")
		assert.Contains(t, module.Inputs, "region")
		assert.Contains(t, module.Inputs, "name")
	})

	t.Run("module with depends_on", func(t *testing.T) {
		dir := t.TempDir()
		mainPath := filepath.Join(dir, "main.tf")
		content := `
module "database" {
  source = "../modules/database"
  name   = "acme-db"
}

module "app" {
  source = "../modules/app"

  depends_on = [module.database]

  db_name = "acme-db"
}
`
		err := os.WriteFile(mainPath, []byte(content), 0644)
		require.NoError(t, err)

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		require.Len(t, calls, 2)

		// depends_on is a meta-argument and shouldn't be in inputs
		for _, module := range calls {
			assert.NotContains(t, module.Inputs, "depends_on")
		}
	})

	t.Run("no variables file", func(t *testing.T) {
		dir := t.TempDir()
		mainPath := filepath.Join(dir, "main.tf")
		content := `
module "vpc" {
  source = "../modules/vpc"
  env    = "staging"
}
`
		err := os.WriteFile(mainPath, []byte(content), 0644)
		require.NoError(t, err)

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		require.Len(t, calls, 1)

		module := calls[0]
		assert.Equal(t, "vpc", module.Name)
		assert.Contains(t, module.Inputs, "env")
	})

	t.Run("variable without default", func(t *testing.T) {
		dir := t.TempDir()

		// Create variables.tf with variable without default
		varsPath := filepath.Join(dir, "variables.tf")
		varsContent := `
variable "env" {
  type        = string
  description = "Environment"
}

variable "region" {
  default = "us-east-1"
}
`
		err := os.WriteFile(varsPath, []byte(varsContent), 0644)
		require.NoError(t, err)

		// Create main.tf
		mainPath := filepath.Join(dir, "main.tf")
		mainContent := `
module "vpc" {
  source = "../modules/vpc"
  env    = var.env
  region = var.region
}
`
		err = os.WriteFile(mainPath, []byte(mainContent), 0644)
		require.NoError(t, err)

		calls, err := ParseModuleCalls(dir)
		require.NoError(t, err)
		require.Len(t, calls, 1)

		module := calls[0]
		assert.Equal(t, "vpc", module.Name)
		// Both variables should still be parsed; var.env gets DynamicVal if no default
		assert.Contains(t, module.Inputs, "env")
		assert.Contains(t, module.Inputs, "region")
	})
}
