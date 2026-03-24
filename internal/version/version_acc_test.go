package version

import (
	"context"
	"testing"

	"dangernoodle.io/terranoodle/internal/testutil"
	"github.com/stretchr/testify/require"
)

// TestAcc_CheckTerraform tests that CheckTerraform calls real terraform binary.
func TestAcc_CheckTerraform(t *testing.T) {
	testutil.SkipUnlessAcc(t)

	err := CheckTerraform(context.Background())
	require.NoError(t, err)
}

// TestAcc_CheckTerragrunt tests that CheckTerragrunt calls real terragrunt binary.
func TestAcc_CheckTerragrunt(t *testing.T) {
	testutil.SkipUnlessAcc(t)

	err := CheckTerragrunt(context.Background())
	require.NoError(t, err)
}
