package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func tfBinary() (string, error) {
	p, err := exec.LookPath("terraform")
	if err != nil {
		return "", fmt.Errorf("importer: terraform binary not found in PATH: %w", err)
	}
	return p, nil
}

func tgBinary() (string, error) {
	p, err := exec.LookPath("terragrunt")
	if err != nil {
		return "", fmt.Errorf("importer: terragrunt binary not found in PATH: %w", err)
	}
	return p, nil
}

// CheckVersion verifies that the terraform binary on PATH is >= 1.5.
func CheckVersion(ctx context.Context, workDir string) error {
	bin, err := tfBinary()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "version", "-json")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("importer: terraform version: %w", err)
	}
	var result struct {
		Version string `json:"terraform_version"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return fmt.Errorf("importer: parse terraform version: %w", err)
	}
	parts := strings.SplitN(result.Version, ".", 3)
	if len(parts) < 2 {
		return fmt.Errorf("importer: unexpected version format: %s", result.Version)
	}
	var major, minor int
	if _, err := fmt.Sscanf(parts[0], "%d", &major); err != nil {
		return fmt.Errorf("importer: parse major version %q: %w", parts[0], err)
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &minor); err != nil {
		return fmt.Errorf("importer: parse minor version %q: %w", parts[1], err)
	}
	if major < 1 || (major == 1 && minor < 5) {
		return fmt.Errorf("importer: terraform %s is below minimum required version 1.5", result.Version)
	}
	return nil
}

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

// TerragruntApply runs terragrunt apply in workDir, applying any import blocks present.
func TerragruntApply(ctx context.Context, workDir string) error {
	bin, err := tgBinary()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "apply", "-auto-approve")
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("importer: terragrunt apply: %w", err)
	}
	return nil
}

// Apply runs terraform apply in workDir, applying any import blocks present.
func Apply(ctx context.Context, workDir string) error {
	bin, err := tfBinary()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "apply", "-auto-approve")
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("importer: terraform apply: %w", err)
	}
	return nil
}

// GeneratePlan runs terraform plan and returns the plan as JSON bytes.
func GeneratePlan(ctx context.Context, workDir string, verbose bool) ([]byte, error) {
	bin, err := tfBinary()
	if err != nil {
		return nil, err
	}

	tmpFile, err := os.CreateTemp("", "terra-tools-plan-*")
	if err != nil {
		return nil, fmt.Errorf("importer: terraform plan: %w", err)
	}
	planPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(planPath)

	planCmd := exec.CommandContext(ctx, bin, "plan", "-out="+planPath)
	planCmd.Dir = workDir
	if verbose {
		planCmd.Stdout = os.Stdout
		planCmd.Stderr = os.Stderr
	}
	if err := planCmd.Run(); err != nil {
		return nil, fmt.Errorf("importer: terraform plan: %w", err)
	}

	showCmd := exec.CommandContext(ctx, bin, "show", "-json", planPath)
	showCmd.Dir = workDir
	out, err := showCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("importer: terraform plan: %w", err)
	}
	return out, nil
}

// TerragruntGeneratePlan runs terragrunt plan and returns the plan as JSON bytes.
func TerragruntGeneratePlan(ctx context.Context, workDir string, verbose bool) ([]byte, error) {
	bin, err := tgBinary()
	if err != nil {
		return nil, err
	}

	tmpFile, err := os.CreateTemp("", "terra-tools-plan-*")
	if err != nil {
		return nil, fmt.Errorf("importer: terragrunt plan: %w", err)
	}
	planPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(planPath)

	planCmd := exec.CommandContext(ctx, bin, "plan", "-out="+planPath)
	planCmd.Dir = workDir
	if verbose {
		planCmd.Stdout = os.Stdout
		planCmd.Stderr = os.Stderr
	}
	if err := planCmd.Run(); err != nil {
		return nil, fmt.Errorf("importer: terragrunt plan: %w", err)
	}

	showCmd := exec.CommandContext(ctx, bin, "show", "-json", planPath)
	showCmd.Dir = workDir
	out, err := showCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("importer: terragrunt plan: %w", err)
	}
	return out, nil
}
