package tfmod

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVariables_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	result, err := ParseVariables(dir)

	require.Nil(t, err)
	assert.Nil(t, result)
}

func TestParseVariables_VariableWithoutDefault(t *testing.T) {
	dir := t.TempDir()
	varFile := filepath.Join(dir, "variables.tf")
	content := `variable "region" {}`
	require.Nil(t, os.WriteFile(varFile, []byte(content), 0o644))

	result, err := ParseVariables(dir)

	require.Nil(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "region", result[0].Name)
	assert.False(t, result[0].HasDefault)
}

func TestParseVariables_VariableWithDefault(t *testing.T) {
	dir := t.TempDir()
	varFile := filepath.Join(dir, "variables.tf")
	content := `variable "env" { default = "staging" }`
	require.Nil(t, os.WriteFile(varFile, []byte(content), 0o644))

	result, err := ParseVariables(dir)

	require.Nil(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "env", result[0].Name)
	assert.True(t, result[0].HasDefault)
}

func TestParseVariables_VariableWithType(t *testing.T) {
	dir := t.TempDir()
	varFile := filepath.Join(dir, "variables.tf")
	content := `variable "name" { type = string }`
	require.Nil(t, os.WriteFile(varFile, []byte(content), 0o644))

	result, err := ParseVariables(dir)

	require.Nil(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "name", result[0].Name)
	assert.NotNil(t, result[0].Type)
}

func TestParseVariables_VariableWithTypeAndDefault(t *testing.T) {
	dir := t.TempDir()
	varFile := filepath.Join(dir, "variables.tf")
	content := `variable "environment" {
  type    = string
  default = "staging"
}`
	require.Nil(t, os.WriteFile(varFile, []byte(content), 0o644))

	result, err := ParseVariables(dir)

	require.Nil(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "environment", result[0].Name)
	assert.True(t, result[0].HasDefault)
	assert.NotNil(t, result[0].Type)
}

func TestParseVariables_MultipleVariablesInFile(t *testing.T) {
	dir := t.TempDir()
	varFile := filepath.Join(dir, "variables.tf")
	content := `variable "region" {
  type    = string
  default = "us-west-2"
}

variable "tags" {
  type = map(string)
}

variable "instance_count" {
  type    = number
  default = 1
}`
	require.Nil(t, os.WriteFile(varFile, []byte(content), 0o644))

	result, err := ParseVariables(dir)

	require.Nil(t, err)
	require.Len(t, result, 3)

	// Check that all variables are present
	varMap := make(map[string]*Variable)
	for i := range result {
		varMap[result[i].Name] = &result[i]
	}

	assert.NotNil(t, varMap["region"])
	assert.True(t, varMap["region"].HasDefault)
	assert.NotNil(t, varMap["region"].Type)

	assert.NotNil(t, varMap["tags"])
	assert.False(t, varMap["tags"].HasDefault)
	assert.NotNil(t, varMap["tags"].Type)

	assert.NotNil(t, varMap["instance_count"])
	assert.True(t, varMap["instance_count"].HasDefault)
	assert.NotNil(t, varMap["instance_count"].Type)
}

func TestParseVariables_MultipleFiles(t *testing.T) {
	dir := t.TempDir()

	// First file
	varFile1 := filepath.Join(dir, "variables.tf")
	content1 := `variable "region" {
  type    = string
  default = "us-west-2"
}`
	require.Nil(t, os.WriteFile(varFile1, []byte(content1), 0o644))

	// Second file
	varFile2 := filepath.Join(dir, "local_variables.tf")
	content2 := `variable "project_id" {
  type = string
}`
	require.Nil(t, os.WriteFile(varFile2, []byte(content2), 0o644))

	result, err := ParseVariables(dir)

	require.Nil(t, err)
	require.Len(t, result, 2)

	// Check that both variables are present
	varMap := make(map[string]*Variable)
	for i := range result {
		varMap[result[i].Name] = &result[i]
	}

	assert.NotNil(t, varMap["region"])
	assert.NotNil(t, varMap["project_id"])
}

func TestParseVariables_NonTfFilesIgnored(t *testing.T) {
	dir := t.TempDir()

	// Write a .tf file
	tfFile := filepath.Join(dir, "variables.tf")
	content := `variable "acme_env" {}`
	require.Nil(t, os.WriteFile(tfFile, []byte(content), 0o644))

	// Write a .hcl file (should be ignored)
	hclFile := filepath.Join(dir, "other.hcl")
	hclContent := `variable "ignored" {}`
	require.Nil(t, os.WriteFile(hclFile, []byte(hclContent), 0o644))

	// Write a .txt file (should be ignored)
	txtFile := filepath.Join(dir, "readme.txt")
	require.Nil(t, os.WriteFile(txtFile, []byte("readme"), 0o644))

	result, err := ParseVariables(dir)

	require.Nil(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "acme_env", result[0].Name)
}

func TestParseVariables_DirectoriesIgnored(t *testing.T) {
	dir := t.TempDir()

	// Write a .tf file
	tfFile := filepath.Join(dir, "variables.tf")
	content := `variable "acme_region" {}`
	require.Nil(t, os.WriteFile(tfFile, []byte(content), 0o644))

	// Create a subdirectory (should be ignored)
	subDir := filepath.Join(dir, "subdir.tf")
	require.Nil(t, os.Mkdir(subDir, 0o755))

	result, err := ParseVariables(dir)

	require.Nil(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "acme_region", result[0].Name)
}

func TestParseVariables_ComplexVariable(t *testing.T) {
	dir := t.TempDir()
	varFile := filepath.Join(dir, "variables.tf")
	content := `variable "acme_config" {
  type = object({
    name   = string
    tags   = map(string)
    ports  = list(number)
  })
  default = {
    name = "acme-service"
    tags = { "Environment" = "staging" }
    ports = [8080, 8443]
  }
  description = "Configuration for acme service"
}`
	require.Nil(t, os.WriteFile(varFile, []byte(content), 0o644))

	result, err := ParseVariables(dir)

	require.Nil(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "acme_config", result[0].Name)
	assert.True(t, result[0].HasDefault)
	assert.True(t, result[0].HasDescription)
	assert.NotNil(t, result[0].Type)
}

func TestParseVariables_HasDescription(t *testing.T) {
	dir := t.TempDir()
	writeFile := func(t *testing.T, path, content string) {
		require.Nil(t, os.WriteFile(path, []byte(content), 0o644))
	}
	writeFile(t, filepath.Join(dir, "variables.tf"), `
variable "with_desc" {
  description = "has a description"
  type        = string
}

variable "without_desc" {
  type = string
}
`)
	result, err := ParseVariables(dir)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "with_desc", result[0].Name)
	assert.True(t, result[0].HasDescription)
	assert.Equal(t, "without_desc", result[1].Name)
	assert.False(t, result[1].HasDescription)
}
