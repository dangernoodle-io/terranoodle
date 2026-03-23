package tfmod

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveModuleDir_EmptyPath(t *testing.T) {
	_, err := ResolveModuleDir("")

	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestResolveModuleDir_ValidDirectory(t *testing.T) {
	dir := t.TempDir()

	result, err := ResolveModuleDir(dir)

	require.Nil(t, err)
	assert.Equal(t, dir, result)
}

func TestResolveModuleDir_FileNotDirectory(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "test*.txt")
	require.Nil(t, err)
	defer file.Close()

	_, err = ResolveModuleDir(file.Name())

	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestResolveModuleDir_NonexistentPath(t *testing.T) {
	_, err := ResolveModuleDir("/nonexistent/path/to/acme-module")

	require.NotNil(t, err)
	assert.NotNil(t, err)
}
