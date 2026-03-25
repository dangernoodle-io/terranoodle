package tfmod

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersionsTF_Missing(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := ParseVersionsTF(tmpDir)
	require.NoError(t, err)
	assert.False(t, result.Exists)
}

func TestParseVersionsTF_NoTerraformBlock(t *testing.T) {
	tmpDir := t.TempDir()

	// Create versions.tf without a terraform block
	content := `# versions.tf without a terraform block
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "versions.tf"), []byte(content), 0o644))

	result, err := ParseVersionsTF(tmpDir)
	require.NoError(t, err)
	assert.True(t, result.Exists)
	assert.False(t, result.HasTerraformBlock)
}

func TestParseVersionsTF_Valid(t *testing.T) {
	tmpDir := t.TempDir()

	content := `terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "versions.tf"), []byte(content), 0o644))

	result, err := ParseVersionsTF(tmpDir)
	require.NoError(t, err)
	assert.True(t, result.Exists)
	assert.True(t, result.HasTerraformBlock)
	require.Len(t, result.Providers, 1)

	p := result.Providers[0]
	assert.Equal(t, "aws", p.Name)
	assert.True(t, p.HasSource)
	assert.True(t, p.HasVersion)
}

func TestParseVersionsTF_MissingSourceAndVersion(t *testing.T) {
	tmpDir := t.TempDir()

	content := `terraform {
  required_providers {
    aws = {}
  }
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "versions.tf"), []byte(content), 0o644))

	result, err := ParseVersionsTF(tmpDir)
	require.NoError(t, err)
	assert.True(t, result.Exists)
	assert.True(t, result.HasTerraformBlock)
	require.Len(t, result.Providers, 1)

	p := result.Providers[0]
	assert.Equal(t, "aws", p.Name)
	assert.False(t, p.HasSource)
	assert.False(t, p.HasVersion)
}

func TestParseVersionsTF_Duplicate(t *testing.T) {
	tmpDir := t.TempDir()

	// Create versions.tf with aws provider
	versionsContent := `terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "versions.tf"), []byte(versionsContent), 0o644))

	// Create main.tf also with aws provider
	mainContent := `terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.tf"), []byte(mainContent), 0o644))

	result, err := ParseVersionsTF(tmpDir)
	require.NoError(t, err)
	assert.True(t, result.Exists)
	assert.True(t, result.HasTerraformBlock)
	require.Len(t, result.Providers, 2)

	// Check that we have 2 aws providers (duplicates)
	var awsCount int
	for _, p := range result.Providers {
		if p.Name == "aws" {
			awsCount++
		}
	}
	assert.Equal(t, 2, awsCount)
}
