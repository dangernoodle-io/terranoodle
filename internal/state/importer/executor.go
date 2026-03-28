package importer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"dangernoodle.io/terranoodle/internal/state/tfexec"
)

// FindTerragruntCache locates the .terragrunt-cache working directory.
// It walks .terragrunt-cache/ recursively and returns the path to the
// innermost directory that contains a .terraform/ subdirectory.
func FindTerragruntCache(dir string) (string, error) {
	cacheRoot := filepath.Join(dir, ".terragrunt-cache")
	if _, err := os.Stat(cacheRoot); err != nil {
		return "", fmt.Errorf("importer: .terragrunt-cache not found in %s", dir)
	}

	var found string
	err := filepath.WalkDir(cacheRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		tfDir := filepath.Join(path, ".terraform")
		if info, statErr := os.Stat(tfDir); statErr == nil && info.IsDir() {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("importer: walk .terragrunt-cache: %w", err)
	}
	if found == "" {
		return "", fmt.Errorf("importer: no initialised working directory found in .terragrunt-cache (run terragrunt init)")
	}
	return found, nil
}

// CheckInit returns an error if workDir has not been initialised.
func CheckInit(workDir string) error {
	info, err := os.Stat(fmt.Sprintf("%s/.terraform", workDir))
	if err != nil || !info.IsDir() {
		return fmt.Errorf("importer: %s has not been initialised (run terraform init)", workDir)
	}
	return nil
}

func apply(ctx context.Context, workDir, bin string) error {
	return tfexec.Run(ctx, bin, workDir, io.Discard, io.Discard, "apply", "-auto-approve")
}

// Apply runs terraform apply in workDir, applying any import blocks present.
func Apply(ctx context.Context, workDir string) error {
	bin, err := tfexec.Binary("terraform")
	if err != nil {
		return fmt.Errorf("importer: %w", err)
	}
	return apply(ctx, workDir, bin)
}

// TerragruntApply runs terragrunt apply in workDir, applying any import blocks present.
func TerragruntApply(ctx context.Context, workDir string) error {
	bin, err := tfexec.Binary("terragrunt")
	if err != nil {
		return fmt.Errorf("importer: %w", err)
	}
	return apply(ctx, workDir, bin)
}

func generatePlan(ctx context.Context, workDir, bin string, verbose bool) ([]byte, error) {
	tmpFile, err := os.CreateTemp("", "terranoodle-plan-*")
	if err != nil {
		return nil, fmt.Errorf("importer: terraform plan: %w", err)
	}
	planPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(planPath)

	var planStdout, planStderr *os.File
	if verbose {
		planStdout = os.Stdout
		planStderr = os.Stderr
	}
	if err := tfexec.Run(ctx, bin, workDir, planStdout, planStderr, "plan", "-out="+planPath); err != nil {
		return nil, fmt.Errorf("importer: terraform plan: %w", err)
	}

	showStdout := &bytes.Buffer{}
	if err := tfexec.Run(ctx, bin, workDir, showStdout, os.Stderr, "show", "-json", planPath); err != nil {
		return nil, fmt.Errorf("importer: terraform plan: %w", err)
	}

	return showStdout.Bytes(), nil
}

// GeneratePlan runs terraform plan and returns the plan as JSON bytes.
func GeneratePlan(ctx context.Context, workDir string, verbose bool) ([]byte, error) {
	bin, err := tfexec.Binary("terraform")
	if err != nil {
		return nil, fmt.Errorf("importer: %w", err)
	}
	return generatePlan(ctx, workDir, bin, verbose)
}

// TerragruntGeneratePlan runs terragrunt plan and returns the plan as JSON bytes.
func TerragruntGeneratePlan(ctx context.Context, workDir string, verbose bool) ([]byte, error) {
	bin, err := tfexec.Binary("terragrunt")
	if err != nil {
		return nil, fmt.Errorf("importer: %w", err)
	}
	return generatePlan(ctx, workDir, bin, verbose)
}

func terraformImport(ctx context.Context, workDir, addr, id, bin string) error {
	return tfexec.Run(ctx, bin, workDir, io.Discard, io.Discard, "import", addr, id)
}

// TerraformImport runs `terraform import <addr> <id>` in workDir.
func TerraformImport(ctx context.Context, workDir, addr, id string) error {
	bin, err := tfexec.Binary("terraform")
	if err != nil {
		return fmt.Errorf("importer: %w", err)
	}
	if err := terraformImport(ctx, workDir, addr, id, bin); err != nil {
		return fmt.Errorf("importer: terraform import: %w", err)
	}
	return nil
}

// TerragruntImport runs `terragrunt import <addr> <id>` in workDir.
func TerragruntImport(ctx context.Context, workDir, addr, id string) error {
	bin, err := tfexec.Binary("terragrunt")
	if err != nil {
		return fmt.Errorf("importer: %w", err)
	}
	if err := terraformImport(ctx, workDir, addr, id, bin); err != nil {
		return fmt.Errorf("importer: terragrunt import: %w", err)
	}
	return nil
}
