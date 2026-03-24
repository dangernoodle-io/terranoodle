package tfexec

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBinary_NotFound sets PATH to empty and verifies error message.
func TestBinary_NotFound(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := Binary("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tfexec:")
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "not found")
}

// TestBinary_Found verifies that a known binary is found.
func TestBinary_Found(t *testing.T) {
	// "go" should be in PATH during tests
	p, err := Binary("go")
	require.NoError(t, err)
	assert.NotEmpty(t, p)
}

// TestRun_Success verifies that a successful command is captured correctly.
func TestRun_Success(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Use echo command to produce simple output
	err := Run(context.Background(), "echo", "", stdout, stderr, "hello")
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "hello")
}

// TestRun_Failure verifies that a failing command returns an error.
func TestRun_Failure(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Use a command that will fail (go run with invalid args)
	err := Run(context.Background(), "go", "", stdout, stderr, "invalid-command-that-does-not-exist")
	require.Error(t, err)
}
