package version

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	goversion "github.com/hashicorp/go-version"
)

const (
	MinTerraform  = "1.5.0"
	MinTerragrunt = "0.90.0"
)

// Testing seams.
var (
	runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, name, args...).Output()
	}
	lookPath = func(file string) (string, error) { return exec.LookPath(file) }
)

// CheckTerraform verifies that terraform is installed and meets the minimum version requirement.
func CheckTerraform(ctx context.Context) error {
	if _, err := lookPath("terraform"); err != nil {
		return fmt.Errorf("terraform binary not found in PATH")
	}

	output, err := runCommand(ctx, "terraform", "version", "-json")
	if err != nil {
		return fmt.Errorf("failed to get terraform version: %w", err)
	}

	var versionData struct {
		TerraformVersion string `json:"terraform_version"`
	}
	if err := json.Unmarshal(output, &versionData); err != nil {
		return fmt.Errorf("failed to parse terraform version output: %w", err)
	}

	ver, err := goversion.NewVersion(versionData.TerraformVersion)
	if err != nil {
		return fmt.Errorf("failed to parse terraform version: %w", err)
	}

	constraint, err := goversion.NewConstraint(">= " + MinTerraform)
	if err != nil {
		return fmt.Errorf("failed to parse version constraint: %w", err)
	}

	if !constraint.Check(ver) {
		return fmt.Errorf("terraform %s is below minimum required version %s", ver, MinTerraform)
	}

	return nil
}

// CheckTerragrunt verifies that terragrunt is installed and meets the minimum version requirement.
func CheckTerragrunt(ctx context.Context) error {
	if _, err := lookPath("terragrunt"); err != nil {
		return fmt.Errorf("terragrunt binary not found in PATH")
	}

	output, err := runCommand(ctx, "terragrunt", "--version")
	if err != nil {
		return fmt.Errorf("failed to get terragrunt version: %w", err)
	}

	// Parse text output: format is "terragrunt version v0.90.0"
	outputStr := strings.TrimSpace(string(output))
	parts := strings.Fields(outputStr)
	if len(parts) < 3 || parts[1] != "version" {
		return fmt.Errorf("unexpected terragrunt version output format: %s", outputStr)
	}

	versionStr := strings.TrimPrefix(parts[2], "v")
	ver, err := goversion.NewVersion(versionStr)
	if err != nil {
		return fmt.Errorf("failed to parse terragrunt version: %w", err)
	}

	constraint, err := goversion.NewConstraint(">= " + MinTerragrunt)
	if err != nil {
		return fmt.Errorf("failed to parse version constraint: %w", err)
	}

	if !constraint.Check(ver) {
		return fmt.Errorf("terragrunt %s is below minimum required version %s", ver, MinTerragrunt)
	}

	return nil
}
