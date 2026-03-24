package rename

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateMv_BinaryNotFound(t *testing.T) {
	t.Setenv("PATH", "")

	err := StateMv(context.Background(), ".", "aws_s3_bucket.old", "aws_s3_bucket.new")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tfexec:")
	assert.Contains(t, err.Error(), "terraform")
}

func TestTerragruntStateMv_BinaryNotFound(t *testing.T) {
	t.Setenv("PATH", "")

	err := TerragruntStateMv(context.Background(), ".", "aws_s3_bucket.old", "aws_s3_bucket.new")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tfexec:")
	assert.Contains(t, err.Error(), "terragrunt")
}
