package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"dangernoodle.io/terranoodle/internal/output"
	"dangernoodle.io/terranoodle/internal/state/plan"
	"dangernoodle.io/terranoodle/internal/state/rename"
	"dangernoodle.io/terranoodle/internal/ui"
)

// state rename flags.
var (
	renameMovedFlag  bool
	renameMvFlag     bool
	renameApplyFlag  bool
	renameDirFlag    string
	renamePlanFlag   string
	renameOutputFlag string
	renameForceFlag  bool
)

var stateMvFn = func(ctx context.Context, workDir, from, to string, useTerragrunt bool) error {
	if useTerragrunt {
		return rename.TerragruntStateMv(ctx, workDir, from, to)
	}
	return rename.StateMv(ctx, workDir, from, to)
}

var confirmCandidatesFn = func(candidates []rename.Candidate, autoConfirm bool) ([]rename.RenamePair, error) {
	return rename.ConfirmCandidates(os.Stdin, os.Stdout, candidates, autoConfirm)
}

var stateRenameCmd = &cobra.Command{
	Use:   "rename",
	Short: "Detect resource renames and generate moved blocks or execute state mv",
	RunE:  runStateRename,
}

func init() {
	stateRenameCmd.Flags().BoolVar(&renameMovedFlag, "moved", false, "Generate moved {} blocks")
	stateRenameCmd.Flags().BoolVar(&renameMvFlag, "mv", false, "Execute terraform/terragrunt state mv commands")
	stateRenameCmd.Flags().BoolVar(&renameApplyFlag, "apply", false, "Execute the operation (default: preview to stdout)")
	stateRenameCmd.Flags().StringVar(&renameDirFlag, "dir", "", "Working directory (default: current directory)")
	stateRenameCmd.Flags().StringVar(&renamePlanFlag, "plan", "", "Path to existing plan JSON (optional)")
	stateRenameCmd.Flags().StringVarP(&renameOutputFlag, "output", "o", "", "Output file path (default: moved.tf)")
	stateRenameCmd.Flags().BoolVar(&renameForceFlag, "force", false, "Overwrite existing output file")

	stateCmd.AddCommand(stateRenameCmd)
}

func runStateRename(cmd *cobra.Command, args []string) error {
	if !renameMovedFlag && !renameMvFlag {
		return fmt.Errorf("state rename: one of --moved or --mv is required")
	}
	if renameMovedFlag && renameMvFlag {
		return fmt.Errorf("state rename: --moved and --mv are mutually exclusive")
	}

	ctx := context.Background()

	// Resolve working environment.
	env, err := resolveWorkEnv(ctx, renameDirFlag, true)
	if err != nil {
		return fmt.Errorf("state rename: %w", err)
	}

	// Generate or load plan.
	planJSON, err := loadOrGeneratePlan(ctx, renamePlanFlag, env)
	if err != nil {
		if renamePlanFlag != "" {
			return fmt.Errorf("state rename: read plan: %w", err)
		}
		return err
	}

	p, err := plan.Parse(bytes.NewReader(planJSON))
	if err != nil {
		return fmt.Errorf("state rename: parse plan: %w", err)
	}

	definite := rename.DetectFromPlan(p)
	candidates := rename.MatchDestroyCreate(p)

	var confirmed []rename.RenamePair
	if len(candidates) > 0 {
		confirmed, err = confirmCandidatesFn(candidates, true)
		if err != nil {
			return err
		}
	}

	pairs := append(definite, confirmed...)
	if len(pairs) == 0 {
		output.Info("No renames detected")
		return nil
	}

	if renameMovedFlag {
		data := rename.GenerateMovedFile(pairs)
		if !renameApplyFlag {
			fmt.Print(string(data))
			return nil
		}
		path, err := rename.WriteMovedFile(env.dir, renameOutputFlag, data, renameForceFlag)
		if err != nil {
			return err
		}
		output.Success("Written: %s", path)
		return nil
	}

	// --mv mode
	if !renameApplyFlag {
		for _, pair := range pairs {
			output.DryRun("%s state mv %s %s", binaryName(env.useTerragrunt), output.Cyan("%s", pair.From), output.Cyan("%s", pair.To))
		}
		return nil
	}

	// For terragrunt, run state mv from project dir (dir), not cache dir (workDir).
	mvDir := env.workDir
	if env.useTerragrunt {
		mvDir = env.dir
	}
	for _, pair := range pairs {
		stop := ui.Spinner(fmt.Sprintf("Moving %s -> %s", pair.From, pair.To))
		err = stateMvFn(ctx, mvDir, pair.From, pair.To, env.useTerragrunt)
		stop()
		if err != nil {
			return err
		}
		output.Item("%s -> %s", pair.From, pair.To)
	}
	output.Success("State moves complete")
	return nil
}
