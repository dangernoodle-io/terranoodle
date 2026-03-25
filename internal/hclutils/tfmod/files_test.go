package tfmod

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListTFFiles_Found(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some .tf files
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.tf"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "variables.tf"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "outputs.tf"), []byte(""), 0o644))

	// Create a non-.tf file
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte(""), 0o644))

	// Create a subdirectory
	require.NoError(t, os.Mkdir(filepath.Join(tmpDir, "subdir"), 0o755))

	files, err := ListTFFiles(tmpDir)
	require.NoError(t, err)
	assert.Len(t, files, 3)

	// Check that all .tf files are present
	nameMap := make(map[string]bool)
	for _, f := range files {
		nameMap[f] = true
	}
	assert.True(t, nameMap["main.tf"])
	assert.True(t, nameMap["variables.tf"])
	assert.True(t, nameMap["outputs.tf"])
	assert.False(t, nameMap["README.md"])
}

func TestListTFFiles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	files, err := ListTFFiles(tmpDir)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestListTFFiles_NonexistentDir(t *testing.T) {
	_, err := ListTFFiles("/nonexistent/acme-module")
	assert.Error(t, err)
}
