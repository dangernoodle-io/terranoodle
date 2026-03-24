package remove

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

func tfBinary() (string, error) {
	p, err := exec.LookPath("terraform")
	if err != nil {
		return "", fmt.Errorf("remove: terraform binary not found in PATH: %w", err)
	}
	return p, nil
}

func tgBinary() (string, error) {
	p, err := exec.LookPath("terragrunt")
	if err != nil {
		return "", fmt.Errorf("remove: terragrunt binary not found in PATH: %w", err)
	}
	return p, nil
}

// StateRm runs `terraform state rm <addr>` in workDir.
func StateRm(ctx context.Context, workDir, addr string) error {
	bin, err := tfBinary()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "state", "rm", addr)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove: terraform state rm: %w", err)
	}
	return nil
}

// TerragruntStateRm runs `terragrunt state rm <addr>` in workDir.
func TerragruntStateRm(ctx context.Context, workDir, addr string) error {
	bin, err := tgBinary()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "state", "rm", addr)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("remove: terragrunt state rm: %w", err)
	}
	return nil
}
