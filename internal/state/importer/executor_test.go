package importer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePlan_BinaryNotFound(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := GeneratePlan(context.Background(), ".", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tfexec:")
	assert.Contains(t, err.Error(), "terraform")
}

func TestTerragruntGeneratePlan_BinaryNotFound(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := TerragruntGeneratePlan(context.Background(), ".", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tfexec:")
	assert.Contains(t, err.Error(), "terragrunt")
}

// TestFindTerragruntCache_FoundWithTerraform tests finding .terraform in cache.
func TestFindTerragruntCache_FoundWithTerraform(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, ".terragrunt-cache", "abc123", "def456")
	tfDir := filepath.Join(cacheDir, ".terraform")

	err := os.MkdirAll(tfDir, 0o755)
	require.NoError(t, err)

	found, err := FindTerragruntCache(tempDir)
	require.NoError(t, err)
	assert.Equal(t, cacheDir, found)
}

// TestFindTerragruntCache_NoCacheDir tests error when .terragrunt-cache doesn't exist.
func TestFindTerragruntCache_NoCacheDir(t *testing.T) {
	tempDir := t.TempDir()

	_, err := FindTerragruntCache(tempDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), ".terragrunt-cache not found")
}

// TestFindTerragruntCache_NoTerraformDir tests error when no .terraform found.
func TestFindTerragruntCache_NoTerraformDir(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, ".terragrunt-cache", "abc123", "def456")

	err := os.MkdirAll(cacheDir, 0o755)
	require.NoError(t, err)

	_, err = FindTerragruntCache(tempDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no initialised working directory")
}

// TestCheckInit_WithTerraform tests success when .terraform exists.
func TestCheckInit_WithTerraform(t *testing.T) {
	tempDir := t.TempDir()
	tfDir := filepath.Join(tempDir, ".terraform")

	err := os.MkdirAll(tfDir, 0o755)
	require.NoError(t, err)

	err = CheckInit(tempDir)
	require.NoError(t, err)
}

// TestCheckInit_WithoutTerraform tests error when .terraform doesn't exist.
func TestCheckInit_WithoutTerraform(t *testing.T) {
	tempDir := t.TempDir()

	err := CheckInit(tempDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has not been initialised")
}
