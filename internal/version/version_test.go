package version

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSeams overrides lookPath and runCommand for the duration of the test.
func stubSeams(t *testing.T, output []byte) {
	t.Helper()
	origLookPath := lookPath
	origRunCommand := runCommand
	t.Cleanup(func() {
		lookPath = origLookPath
		runCommand = origRunCommand
	})
	lookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return output, nil
	}
}

// TestCheckTerraform_OK tests that terraform with a valid version passes.
func TestCheckTerraform_OK(t *testing.T) {
	stubSeams(t, []byte(`{"terraform_version": "1.9.0"}`))

	err := CheckTerraform(context.Background())
	assert.NoError(t, err)
}

// TestCheckTerraform_BelowMinimum tests that terraform below minimum version returns an error.
func TestCheckTerraform_BelowMinimum(t *testing.T) {
	stubSeams(t, []byte(`{"terraform_version": "1.4.0"}`))

	err := CheckTerraform(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "below minimum")
}

// TestCheckTerraform_ExactMinimum tests that terraform at exact minimum version passes.
func TestCheckTerraform_ExactMinimum(t *testing.T) {
	stubSeams(t, []byte(`{"terraform_version": "1.5.0"}`))

	err := CheckTerraform(context.Background())
	assert.NoError(t, err)
}

// TestCheckTerraform_BinaryNotFound tests that missing terraform binary returns an error.
func TestCheckTerraform_BinaryNotFound(t *testing.T) {
	origLookPath := lookPath
	t.Cleanup(func() { lookPath = origLookPath })
	lookPath = func(file string) (string, error) { return "", fmt.Errorf("not found") }

	err := CheckTerraform(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestCheckTerragrunt_OK tests that terragrunt with a valid version passes.
func TestCheckTerragrunt_OK(t *testing.T) {
	stubSeams(t, []byte("terragrunt version v0.95.0\n"))

	err := CheckTerragrunt(context.Background())
	assert.NoError(t, err)
}

// TestCheckTerragrunt_BelowMinimum tests that terragrunt below minimum version returns an error.
func TestCheckTerragrunt_BelowMinimum(t *testing.T) {
	stubSeams(t, []byte("terragrunt version v0.80.0\n"))

	err := CheckTerragrunt(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "below minimum")
}

// TestCheckTerragrunt_ExactMinimum tests that terragrunt at exact minimum version passes.
func TestCheckTerragrunt_ExactMinimum(t *testing.T) {
	stubSeams(t, []byte("terragrunt version v0.90.0\n"))

	err := CheckTerragrunt(context.Background())
	assert.NoError(t, err)
}

// TestCheckTerragrunt_BinaryNotFound tests that missing terragrunt binary returns an error.
func TestCheckTerragrunt_BinaryNotFound(t *testing.T) {
	origLookPath := lookPath
	t.Cleanup(func() { lookPath = origLookPath })
	lookPath = func(file string) (string, error) { return "", fmt.Errorf("not found") }

	err := CheckTerragrunt(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
