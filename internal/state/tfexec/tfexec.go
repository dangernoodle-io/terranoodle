package tfexec

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// Binary looks up a binary (e.g. "terraform", "terragrunt") in PATH.
func Binary(name string) (string, error) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("tfexec: %s binary not found in PATH: %w", name, err)
	}
	return p, nil
}

// Run executes bin with args in workDir, wiring stdout/stderr to the provided writers.
func Run(ctx context.Context, bin, workDir string, stdout, stderr io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
