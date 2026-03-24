package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dangernoodle.io/terranoodle/internal/state/resolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateImportsFile_SingleEntry(t *testing.T) {
	entries := []resolver.ImportEntry{
		{Address: "aws_s3_bucket.example", ID: "acme-bucket", Type: "aws_s3_bucket"},
	}

	data := GenerateImportsFile(entries)
	content := string(data)

	assert.Contains(t, content, "import {")
	assert.Contains(t, content, "to = aws_s3_bucket.example")
	assert.Contains(t, content, `id = "acme-bucket"`)
	assert.Contains(t, content, "}")
}

func TestGenerateImportsFile_MultipleEntries(t *testing.T) {
	entries := []resolver.ImportEntry{
		{Address: "aws_s3_bucket.bucket1", ID: "bucket-1", Type: "aws_s3_bucket"},
		{Address: "aws_iam_role.role1", ID: "arn:aws:iam::123456789:role/example", Type: "aws_iam_role"},
	}

	data := GenerateImportsFile(entries)
	content := string(data)

	// Check both entries are present and separated by blank line
	lines := strings.Split(content, "\n")
	assert.Greater(t, len(lines), 0)

	assert.Contains(t, content, "aws_s3_bucket.bucket1")
	assert.Contains(t, content, `id = "bucket-1"`)
	assert.Contains(t, content, "aws_iam_role.role1")
	assert.Contains(t, content, `id = "arn:aws:iam::123456789:role/example"`)
}

func TestGenerateImportsFile_EmptySlice(t *testing.T) {
	data := GenerateImportsFile([]resolver.ImportEntry{})
	assert.Equal(t, []byte(nil), data)
}

func TestGenerateImportsFile_NilSlice(t *testing.T) {
	data := GenerateImportsFile(nil)
	assert.Equal(t, []byte(nil), data)
}

func TestWriteImportsFile_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	data := []byte("import {\n  to = aws_s3_bucket.example\n  id = \"bucket\"\n}\n")

	path, err := WriteImportsFile(tmpDir, "", data, false)
	require.NoError(t, err)

	assert.True(t, strings.HasSuffix(path, "imports.tf"))
	assert.True(t, strings.Contains(path, tmpDir))

	// Verify file exists and has correct content
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, content)
}

func TestWriteImportsFile_ExistsNoForce(t *testing.T) {
	tmpDir := t.TempDir()
	importsPath := filepath.Join(tmpDir, "imports.tf")

	// Create file first
	err := os.WriteFile(importsPath, []byte("existing content"), 0o644)
	require.NoError(t, err)

	newData := []byte("new content")
	_, err = WriteImportsFile(tmpDir, "", newData, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Verify original content unchanged
	content, err := os.ReadFile(importsPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("existing content"), content)
}

func TestWriteImportsFile_ExistsWithForce(t *testing.T) {
	tmpDir := t.TempDir()
	importsPath := filepath.Join(tmpDir, "imports.tf")

	// Create file first
	err := os.WriteFile(importsPath, []byte("existing content"), 0o644)
	require.NoError(t, err)

	newData := []byte("new content")
	path, err := WriteImportsFile(tmpDir, "", newData, true)
	require.NoError(t, err)

	assert.Equal(t, importsPath, path)

	// Verify content was overwritten
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, newData, content)
}

func TestRemoveImportsFile_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	importsPath := filepath.Join(tmpDir, "imports.tf")

	// Create file
	err := os.WriteFile(importsPath, []byte("content"), 0o644)
	require.NoError(t, err)

	// Remove it
	err = RemoveImportsFile(importsPath)
	require.NoError(t, err)

	// Verify it's gone
	_, err = os.Stat(importsPath)
	assert.True(t, os.IsNotExist(err))
}

func TestRemoveImportsFile_NotExists(t *testing.T) {
	nonExistentPath := "/nonexistent/path/imports.tf"
	err := RemoveImportsFile(nonExistentPath)
	assert.NoError(t, err)
}
