package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"dangernoodle.io/terranoodle/internal/config"
	"dangernoodle.io/terranoodle/internal/lint/report"
	"dangernoodle.io/terranoodle/internal/lint/validate"
	"dangernoodle.io/terranoodle/internal/output"
)

var (
	lintDirFlag    string
	lintAllFlag    bool
	lintConfigFlag string
	lintStrictFlag bool
)

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Lint terragrunt stack configs",
	RunE:  runLint,
}

func init() {
	lintCmd.Flags().StringVarP(&lintDirFlag, "dir", "d", "", "Directory to lint (default: current directory)")
	lintCmd.Flags().BoolVar(&lintAllFlag, "all", false, "Lint all subdirectories")
	lintCmd.Flags().StringVar(&lintConfigFlag, "config", "", "Path to config file (default: auto-discover)")
	lintCmd.Flags().BoolVar(&lintStrictFlag, "strict", false, "Treat warnings as errors")
}

func runLint(cmd *cobra.Command, args []string) error {
	dir := lintDirFlag
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("lint: get working directory: %w", err)
		}
		dir = cwd
	}

	var cfg *config.Config
	var cfgErr error

	if lintConfigFlag != "" {
		cfg, cfgErr = config.Load(lintConfigFlag)
	} else {
		cfg, cfgErr = config.Discover(dir)
	}
	if cfgErr != nil {
		return cfgErr
	}

	strict := lintStrictFlag || os.Getenv("CI") != ""
	opts := validate.Options{Config: &cfg.Lint, Strict: strict}

	var errs []validate.Error
	var err error

	if lintAllFlag {
		errs, err = validate.WalkDir(dir, opts)
	} else {
		errs, err = validate.Dir(dir, opts)
	}
	if err != nil {
		return err
	}

	var errorCount, warningCount int
	for _, e := range errs {
		if e.Severity == validate.SeverityError {
			errorCount++
		} else {
			warningCount++
		}
	}

	if len(errs) > 0 {
		report.Print(os.Stdout, errs)
		if errorCount > 0 {
			if warningCount > 0 && strict {
				return fmt.Errorf("lint: found %d error(s), %d warning(s)", errorCount, warningCount)
			}
			return fmt.Errorf("lint: found %d error(s)", errorCount)
		}
		if warningCount > 0 && strict {
			return fmt.Errorf("lint: found %d warning(s)", warningCount)
		}
		return nil
	}

	output.Success("No issues found")
	return nil
}
