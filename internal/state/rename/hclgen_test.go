package rename

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateMovedFile_SinglePair(t *testing.T) {
	pairs := []RenamePair{
		{From: "aws_s3_bucket.old", To: "module.storage.aws_s3_bucket.old"},
	}

	data := GenerateMovedFile(pairs)
	content := string(data)

	assert.Contains(t, content, "moved {")
	assert.Contains(t, content, "from = aws_s3_bucket.old")
	assert.Contains(t, content, "to   = module.storage.aws_s3_bucket.old")
	assert.Contains(t, content, "}")
}

func TestGenerateMovedFile_MultiplePairs(t *testing.T) {
	pairs := []RenamePair{
		{From: "aws_s3_bucket.data", To: "module.storage.aws_s3_bucket.data"},
		{From: "aws_iam_role.app", To: "module.iam.aws_iam_role.app"},
	}

	data := GenerateMovedFile(pairs)
	content := string(data)

	assert.Contains(t, content, "aws_s3_bucket.data")
	assert.Contains(t, content, "module.storage.aws_s3_bucket.data")
	assert.Contains(t, content, "aws_iam_role.app")
	assert.Contains(t, content, "module.iam.aws_iam_role.app")
}

func TestGenerateMovedFile_SortsByFromAddress(t *testing.T) {
	pairs := []RenamePair{
		{From: "aws_s3_bucket.zebra", To: "module.z.aws_s3_bucket.zebra"},
		{From: "aws_iam_role.alpha", To: "module.a.aws_iam_role.alpha"},
	}

	data := GenerateMovedFile(pairs)
	content := string(data)

	// alpha should come before zebra in the output
	alphaIdx := len("moved {\n  from = aws_iam_role.alpha")
	zebraIdx := len(content) - 1
	_ = alphaIdx
	_ = zebraIdx

	// Simpler: check that the content starts with the alpha entry
	assert.True(t, len(content) > 0)
	assert.Equal(t, "moved {\n  from = aws_iam_role.alpha\n  to   = module.a.aws_iam_role.alpha\n}\n\nmoved {\n  from = aws_s3_bucket.zebra\n  to   = module.z.aws_s3_bucket.zebra\n}\n", content)
}

func TestGenerateMovedFile_EmptySlice(t *testing.T) {
	data := GenerateMovedFile([]RenamePair{})
	assert.Nil(t, data)
}

func TestGenerateMovedFile_NilSlice(t *testing.T) {
	data := GenerateMovedFile(nil)
	assert.Nil(t, data)
}

func TestWriteMovedFile_DefaultPath(t *testing.T) {
	tmpDir := t.TempDir()
	data := []byte("moved {\n  from = aws_s3_bucket.old\n  to   = aws_s3_bucket.new\n}\n")

	path, err := WriteMovedFile(tmpDir, "", data, false)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, "moved.tf"), path)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, content)
}

func TestWriteMovedFile_CustomPath(t *testing.T) {
	tmpDir := t.TempDir()
	customPath := filepath.Join(tmpDir, "custom-moved.tf")
	data := []byte("moved {\n  from = aws_s3_bucket.old\n  to   = aws_s3_bucket.new\n}\n")

	path, err := WriteMovedFile(tmpDir, customPath, data, false)
	require.NoError(t, err)
	assert.Equal(t, customPath, path)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, content)
}

func TestWriteMovedFile_ExistsNoForce(t *testing.T) {
	tmpDir := t.TempDir()
	movedPath := filepath.Join(tmpDir, "moved.tf")

	err := os.WriteFile(movedPath, []byte("existing content"), 0o644)
	require.NoError(t, err)

	_, err = WriteMovedFile(tmpDir, "", []byte("new content"), false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	content, err := os.ReadFile(movedPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("existing content"), content)
}

func TestWriteMovedFile_ExistsWithForce(t *testing.T) {
	tmpDir := t.TempDir()
	movedPath := filepath.Join(tmpDir, "moved.tf")

	err := os.WriteFile(movedPath, []byte("existing content"), 0o644)
	require.NoError(t, err)

	newData := []byte("new content")
	path, err := WriteMovedFile(tmpDir, "", newData, true)
	require.NoError(t, err)
	assert.Equal(t, movedPath, path)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, newData, content)
}
