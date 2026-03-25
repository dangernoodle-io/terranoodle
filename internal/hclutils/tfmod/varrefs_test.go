package tfmod

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCollectVarRefs_Found(t *testing.T) {
	tmpDir := t.TempDir()

	// Create variables.tf
	varContent := `variable "used_var" {
  description = "A used variable"
  type        = string
}

variable "unused_var" {
  description = "An unused variable"
  type        = string
}`
	require.NoError(t, writeFile(tmpDir, "variables.tf", varContent))

	// Create main.tf that references used_var
	mainContent := `resource "null_resource" "example" {
  triggers = {
    value = var.used_var
  }
}`
	require.NoError(t, writeFile(tmpDir, "main.tf", mainContent))

	refs, err := CollectVarRefs(tmpDir)
	require.NoError(t, err)
	require.True(t, refs["used_var"])
	require.False(t, refs["unused_var"])
}

func TestCollectVarRefs_NoRefs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create only variables.tf with no references
	varContent := `variable "test_var" {
  description = "A test variable"
  type        = string
}`
	require.NoError(t, writeFile(tmpDir, "variables.tf", varContent))

	refs, err := CollectVarRefs(tmpDir)
	require.NoError(t, err)
	require.Empty(t, refs)
}

func TestCollectVarRefs_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	refs, err := CollectVarRefs(tmpDir)
	require.NoError(t, err)
	require.Empty(t, refs)
}

func TestCollectVarRefs_MultiFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create variables.tf
	varContent := `variable "var_a" {
  description = "Variable A"
  type        = string
}

variable "var_b" {
  description = "Variable B"
  type        = string
}

variable "var_c" {
  description = "Variable C"
  type        = string
}`
	require.NoError(t, writeFile(tmpDir, "variables.tf", varContent))

	// Create main.tf referencing var_a
	mainContent := `resource "null_resource" "main" {
  triggers = {
    a = var.var_a
  }
}`
	require.NoError(t, writeFile(tmpDir, "main.tf", mainContent))

	// Create outputs.tf referencing var_b
	outputContent := `output "result" {
  description = "Result"
  value       = var.var_b
}`
	require.NoError(t, writeFile(tmpDir, "outputs.tf", outputContent))

	refs, err := CollectVarRefs(tmpDir)
	require.NoError(t, err)
	require.True(t, refs["var_a"])
	require.True(t, refs["var_b"])
	require.False(t, refs["var_c"])
}

// writeFile writes content to a named file in dir.
func writeFile(dir, name, content string) error {
	return writeFileWithPath(filepath.Join(dir, name), content)
}

// writeFileWithPath writes content to the given path.
func writeFileWithPath(path, content string) error {
	return writeFileBytes(path, []byte(content))
}

// writeFileBytes writes raw bytes to the given path.
func writeFileBytes(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
