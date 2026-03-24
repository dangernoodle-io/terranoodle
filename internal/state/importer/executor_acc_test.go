package importer

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"dangernoodle.io/terranoodle/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAcc_GeneratePlan tests that GeneratePlan runs terraform plan and returns JSON.
func TestAcc_GeneratePlan(t *testing.T) {
	testutil.SkipUnlessAcc(t)

	tmpDir := t.TempDir()
	err := copyMinimalTestdata(tmpDir)
	require.NoError(t, err)

	err = runTerraformInit(t, tmpDir)
	require.NoError(t, err)

	planJSON, err := GeneratePlan(context.Background(), tmpDir, false)
	require.NoError(t, err)

	// Verify the JSON contains resource_changes
	var plan map[string]interface{}
	err = json.Unmarshal(planJSON, &plan)
	require.NoError(t, err)
	assert.Contains(t, plan, "resource_changes")
}

// TestAcc_CheckInit tests that CheckInit validates terraform init.
func TestAcc_CheckInit(t *testing.T) {
	testutil.SkipUnlessAcc(t)

	tmpDir := t.TempDir()
	err := copyMinimalTestdata(tmpDir)
	require.NoError(t, err)

	err = runTerraformInit(t, tmpDir)
	require.NoError(t, err)

	err = CheckInit(tmpDir)
	require.NoError(t, err)
}

// TestAcc_CheckState tests that CheckState returns no error on initialized dir.
func TestAcc_CheckState(t *testing.T) {
	testutil.SkipUnlessAcc(t)

	tmpDir := t.TempDir()
	err := copyMinimalTestdata(tmpDir)
	require.NoError(t, err)

	err = runTerraformInit(t, tmpDir)
	require.NoError(t, err)

	alreadyManaged, err := CheckState(context.Background(), tmpDir, []string{}, false)
	require.NoError(t, err)
	assert.Nil(t, alreadyManaged)
}

// copyMinimalTestdata copies minimal/main.tf from testdata to tmpDir.
func copyMinimalTestdata(tmpDir string) error {
	minimalDir := filepath.Join("..", "..", "testutil", "testdata", "minimal")
	srcFile := filepath.Join(minimalDir, "main.tf")

	content, err := os.ReadFile(srcFile)
	if err != nil {
		return err
	}

	dstFile := filepath.Join(tmpDir, "main.tf")
	return os.WriteFile(dstFile, content, 0o644)
}

// runTerraformInit runs terraform init in the given directory.
func runTerraformInit(t *testing.T, workDir string) error {
	bin, err := exec.LookPath("terraform")
	if err != nil {
		t.Skip("terraform binary not found in PATH")
	}

	cmd := exec.Command(bin, "init")
	cmd.Dir = workDir
	return cmd.Run()
}
