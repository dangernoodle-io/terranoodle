package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"dangernoodle.io/terra-tools/internal/lint/report"
	"dangernoodle.io/terra-tools/internal/lint/validate"
	"dangernoodle.io/terra-tools/internal/output"
	"github.com/spf13/cobra"
)

var lintCmd = &cobra.Command{
	Use:   "lint [flags] [path...]",
	Short: "Lint terragrunt configurations",
	Long: `Validate terragrunt inputs and terraform module calls against module variables
— catching missing required inputs, extra inputs, and type mismatches before
plan/apply.

Arguments:
  path    terragrunt.hcl, terragrunt.stack.hcl, .tf file, or directory

Environment:
  SKIP_LINT   set to any value to skip linting (exit 0 immediately)`,
	RunE: runLint,
}

var lintRecursive bool

func init() {
	lintCmd.Flags().BoolVarP(&lintRecursive, "recursive", "r", false, "Recursively walk directories")
}

func runLint(cmd *cobra.Command, args []string) error {
	if os.Getenv("SKIP_LINT") != "" {
		output.Warn("terra-tools lint: skipped (SKIP_LINT set)")
		return nil
	}

	if len(args) == 0 {
		return cmd.Help()
	}

	var allErrors []validate.Error

	for _, path := range args {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("cannot access %s: %w", path, err)
		}

		var errs []validate.Error
		var toolErr error
		if info.IsDir() {
			if lintRecursive {
				errs, toolErr = validate.WalkDir(path)
			} else {
				errs, toolErr = validate.Dir(path)
			}
		} else if filepath.Base(path) == "terragrunt.stack.hcl" {
			errs, toolErr = validate.StackFile(path)
		} else if filepath.Ext(path) == ".tf" {
			errs, toolErr = validate.TerraformDir(filepath.Dir(path))
		} else {
			errs, toolErr = validate.File(path)
		}
		if toolErr != nil {
			return toolErr
		}
		allErrors = append(allErrors, errs...)
	}

	if len(allErrors) > 0 {
		report.Print(os.Stdout, allErrors)
		os.Exit(1)
	}
	output.Success("terra-tools lint: all checks passed")
	return nil
}
