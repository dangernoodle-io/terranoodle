package cli

import (
	"bytes"
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"dangernoodle.io/terranoodle/internal/output"
	"dangernoodle.io/terranoodle/internal/state/plan"
	"dangernoodle.io/terranoodle/internal/state/remove"
	"dangernoodle.io/terranoodle/internal/ui"
)

// state remove flags.
var (
	removeDirFlag   string
	removePlanFlag  string
	removeApplyFlag bool
)

var stateRmFn = func(ctx context.Context, workDir, addr string, useTerragrunt bool) error {
	if useTerragrunt {
		return remove.TerragruntStateRm(ctx, workDir, addr)
	}
	return remove.StateRm(ctx, workDir, addr)
}

var stateRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove destroyed resources from state without destroying infrastructure",
	RunE:  runStateRemove,
}

func init() {
	stateRemoveCmd.Flags().StringVar(&removeDirFlag, "dir", "", "Working directory (default: current directory)")
	stateRemoveCmd.Flags().StringVar(&removePlanFlag, "plan", "", "Path to existing plan JSON (optional)")
	stateRemoveCmd.Flags().BoolVar(&removeApplyFlag, "apply", false, "Execute state rm commands (default: preview)")

	stateCmd.AddCommand(stateRemoveCmd)
}

func runStateRemove(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Resolve working environment.
	env, err := resolveWorkEnv(ctx, removeDirFlag, true)
	if err != nil {
		return fmt.Errorf("state remove: %w", err)
	}

	// Generate or load plan.
	planJSON, err := loadOrGeneratePlan(ctx, removePlanFlag, env)
	if err != nil {
		if removePlanFlag != "" {
			return fmt.Errorf("state remove: read plan: %w", err)
		}
		return err
	}

	p, err := plan.Parse(bytes.NewReader(planJSON))
	if err != nil {
		return fmt.Errorf("state remove: parse plan: %w", err)
	}

	targets := remove.DetectFromPlan(p)
	if len(targets) == 0 {
		output.Info("No resources to remove from state")
		return nil
	}

	if !removeApplyFlag {
		for _, t := range targets {
			output.DryRun("%s state rm %s", binaryName(env.useTerragrunt), output.Cyan("%s", t.Address))
		}
		return nil
	}

	// Apply mode: run state rm for each target.
	rmDir := env.workDir
	if env.useTerragrunt {
		rmDir = env.dir
	}
	for _, t := range targets {
		stop := ui.Spinner(fmt.Sprintf("Removing %s", t.Address))
		err = stateRmFn(ctx, rmDir, t.Address, env.useTerragrunt)
		stop()
		if err != nil {
			return err
		}
		output.Item("%s", t.Address)
	}
	output.Success("State removals complete")
	return nil
}
