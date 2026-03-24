package rename

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

func tfBinary() (string, error) {
	p, err := exec.LookPath("terraform")
	if err != nil {
		return "", fmt.Errorf("rename: terraform binary not found in PATH: %w", err)
	}
	return p, nil
}

func tgBinary() (string, error) {
	p, err := exec.LookPath("terragrunt")
	if err != nil {
		return "", fmt.Errorf("rename: terragrunt binary not found in PATH: %w", err)
	}
	return p, nil
}

// StateMv runs `terraform state mv <from> <to>` in workDir.
func StateMv(ctx context.Context, workDir, from, to string) error {
	bin, err := tfBinary()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "state", "mv", from, to)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rename: terraform state mv: %w", err)
	}
	return nil
}

// TerragruntStateMv runs `terragrunt state mv <from> <to>` in workDir.
func TerragruntStateMv(ctx context.Context, workDir, from, to string) error {
	bin, err := tgBinary()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "state", "mv", from, to)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rename: terragrunt state mv: %w", err)
	}
	return nil
}
