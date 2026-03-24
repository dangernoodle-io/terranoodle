package rename

import (
	"context"
	"fmt"
	"os"

	"dangernoodle.io/terranoodle/internal/state/tfexec"
)

func stateMv(ctx context.Context, workDir, src, dst, bin string) error {
	return tfexec.Run(ctx, bin, workDir, os.Stdout, os.Stderr, "state", "mv", src, dst)
}

// StateMv runs `terraform state mv <from> <to>` in workDir.
func StateMv(ctx context.Context, workDir, from, to string) error {
	bin, err := tfexec.Binary("terraform")
	if err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	if err := stateMv(ctx, workDir, from, to, bin); err != nil {
		return fmt.Errorf("rename: terraform state mv: %w", err)
	}
	return nil
}

// TerragruntStateMv runs `terragrunt state mv <from> <to>` in workDir.
func TerragruntStateMv(ctx context.Context, workDir, from, to string) error {
	bin, err := tfexec.Binary("terragrunt")
	if err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	if err := stateMv(ctx, workDir, from, to, bin); err != nil {
		return fmt.Errorf("rename: terragrunt state mv: %w", err)
	}
	return nil
}
