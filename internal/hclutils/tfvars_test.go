package hclutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTfVarKeys_ValidFile(t *testing.T) {
	dir := t.TempDir()
	tfvarsFile := filepath.Join(dir, "terraform.tfvars")
	content := `
environment = "staging"
region      = "us-west-2"
tags = {
  owner = "test-user"
  cost_center = "acme-corp"
}
`
	err := os.WriteFile(tfvarsFile, []byte(content), 0644)
	require.NoError(t, err)

	keys := ParseTfVarKeys([]string{tfvarsFile})
	assert.True(t, keys["environment"])
	assert.True(t, keys["region"])
	assert.True(t, keys["tags"])
	assert.Len(t, keys, 3)
}

func TestParseTfVarKeys_MissingFile(t *testing.T) {
	dir := t.TempDir()
	missingFile := filepath.Join(dir, "nonexistent.tfvars")

	// Should silently skip missing files
	keys := ParseTfVarKeys([]string{missingFile})
	assert.Empty(t, keys)
}

func TestParseTfVarKeys_Mixed(t *testing.T) {
	dir := t.TempDir()

	// Create first file
	tfvarsFile1 := filepath.Join(dir, "terraform.tfvars")
	content1 := `
environment = "prod"
region      = "us-east-1"
`
	err := os.WriteFile(tfvarsFile1, []byte(content1), 0644)
	require.NoError(t, err)

	// Create second file
	tfvarsFile2 := filepath.Join(dir, "custom.auto.tfvars")
	content2 := `
project_name = "example-project"
enabled      = true
`
	err = os.WriteFile(tfvarsFile2, []byte(content2), 0644)
	require.NoError(t, err)

	// Missing file path
	missingFile := filepath.Join(dir, "missing.tfvars")

	// Parse both existing and missing
	keys := ParseTfVarKeys([]string{tfvarsFile1, tfvarsFile2, missingFile})
	assert.True(t, keys["environment"])
	assert.True(t, keys["region"])
	assert.True(t, keys["project_name"])
	assert.True(t, keys["enabled"])
	assert.Len(t, keys, 4)
}

func TestParseTfVarKeys_InvalidHCL(t *testing.T) {
	dir := t.TempDir()
	tfvarsFile := filepath.Join(dir, "invalid.tfvars")
	content := `
environment = "staging
  # missing closing quote
`
	err := os.WriteFile(tfvarsFile, []byte(content), 0644)
	require.NoError(t, err)

	// Should silently skip invalid files
	keys := ParseTfVarKeys([]string{tfvarsFile})
	assert.Empty(t, keys)
}

func TestParseTfVarKeys_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	tfvarsFile := filepath.Join(dir, "empty.tfvars")
	err := os.WriteFile(tfvarsFile, []byte(""), 0644)
	require.NoError(t, err)

	keys := ParseTfVarKeys([]string{tfvarsFile})
	assert.Empty(t, keys)
}

func TestParseTfVarKeys_ComplexTypes(t *testing.T) {
	dir := t.TempDir()
	tfvarsFile := filepath.Join(dir, "complex.tfvars")
	content := `
simple_string = "value"
number_var    = 42
bool_var      = true
list_var      = ["a", "b", "c"]
map_var = {
  key1 = "val1"
  key2 = "val2"
}
`
	err := os.WriteFile(tfvarsFile, []byte(content), 0644)
	require.NoError(t, err)

	keys := ParseTfVarKeys([]string{tfvarsFile})
	assert.True(t, keys["simple_string"])
	assert.True(t, keys["number_var"])
	assert.True(t, keys["bool_var"])
	assert.True(t, keys["list_var"])
	assert.True(t, keys["map_var"])
	assert.Len(t, keys, 5)
}

func TestParseTfVarKeys_EmptyList(t *testing.T) {
	keys := ParseTfVarKeys([]string{})
	assert.Empty(t, keys)
}
