package remove

import (
	"context"
	"fmt"
	"os"

	"dangernoodle.io/terranoodle/internal/state/tfexec"
)

func stateRm(ctx context.Context, workDir, addr, bin string) error {
	return tfexec.Run(ctx, bin, workDir, os.Stdout, os.Stderr, "state", "rm", addr)
}

// StateRm runs `terraform state rm <addr>` in workDir.
func StateRm(ctx context.Context, workDir, addr string) error {
	bin, err := tfexec.Binary("terraform")
	if err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	if err := stateRm(ctx, workDir, addr, bin); err != nil {
		return fmt.Errorf("remove: terraform state rm: %w", err)
	}
	return nil
}

// TerragruntStateRm runs `terragrunt state rm <addr>` in workDir.
func TerragruntStateRm(ctx context.Context, workDir, addr string) error {
	bin, err := tfexec.Binary("terragrunt")
	if err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	if err := stateRm(ctx, workDir, addr, bin); err != nil {
		return fmt.Errorf("remove: terragrunt state rm: %w", err)
	}
	return nil
}
