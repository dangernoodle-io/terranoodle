package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	profileconfig "dangernoodle.io/terranoodle/internal/config"
	"dangernoodle.io/terranoodle/internal/output"
	"dangernoodle.io/terranoodle/internal/state/plan"
	"dangernoodle.io/terranoodle/internal/state/scaffold"
	"dangernoodle.io/terranoodle/internal/state/scaffold/store"
	"dangernoodle.io/terranoodle/internal/ui"
)

// state scaffold flags.
var (
	scaffoldDirFlag           string
	scaffoldOutputFlag        string
	scaffoldFetchRegistryFlag bool
	scaffoldSaveFlag          bool
)

var stateScaffoldCmd = &cobra.Command{
	Use:   "scaffold",
	Short: "Scaffold an import config from a terraform plan",
	Long: `Scaffold an import config YAML from resources in a terraform plan.

By default, the id field for each resource type is set to "TODO" and must
be filled in manually. Pass --fetch-registry to auto-populate id templates
by looking up the import format from the Terraform provider documentation.

Known templates from central scaffold state (~/.config/terranoodle/scaffold/state/)
are automatically used when available. Use --save to persist new templates
back to central state for future use.

Example:

  terranoodle state scaffold                          # id: "TODO"
  terranoodle state scaffold --fetch-registry         # id: "projects/{{ .project }}/..."
  terranoodle state scaffold --fetch-registry --save  # fetch + save to central state`,
	RunE: runStateScaffold,
}

func init() {
	stateScaffoldCmd.Flags().StringVar(&scaffoldDirFlag, "dir", "", "Working directory (default: current directory)")
	stateScaffoldCmd.Flags().StringVarP(&scaffoldOutputFlag, "output", "o", "", "Output file (default: stdout)")
	stateScaffoldCmd.Flags().BoolVar(&scaffoldFetchRegistryFlag, "fetch-registry", false, "Fetch import formats from the Terraform registry")
	stateScaffoldCmd.Flags().BoolVar(&scaffoldSaveFlag, "save", false, "Save type templates to central scaffold state")

	stateCmd.AddCommand(stateScaffoldCmd)
}

func runStateScaffold(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Resolve working environment (requireInit=false for scaffold).
	env, err := resolveWorkEnv(ctx, scaffoldDirFlag, false)
	if err != nil {
		return fmt.Errorf("state scaffold: %w", err)
	}

	// Generate plan. For terragrunt, run from project dir, not cache dir.
	planDir := env.workDir
	if env.useTerragrunt {
		planDir = env.dir
	}
	var planJSON []byte
	stopSpinner := ui.Spinner("Generating plan...")
	planJSON, err = generatePlanJSONFn(ctx, planDir, env.useTerragrunt)
	stopSpinner()
	if err != nil {
		return err
	}

	p, err := plan.Parse(bytes.NewReader(planJSON))
	if err != nil {
		return fmt.Errorf("state scaffold: parse plan: %w", err)
	}

	creates := plan.FilterCreates(p)
	if len(creates) == 0 {
		output.Info("No resources to scaffold")
		return nil
	}

	var formats map[string]string
	if scaffoldFetchRegistryFlag {
		cache := map[string]string{}
		formats = map[string]string{}

		// Collect unique resource types.
		seen := map[string]bool{}
		for _, c := range creates {
			if !seen[c.Type] {
				seen[c.Type] = true
				rt := c.Type
				stop := ui.Spinner("Fetching docs for " + rt)
				formats[rt] = scaffold.FetchImportFormat(ctx, rt, cache)
				stop()
			}
		}
	}

	types := scaffold.Generate(creates, formats)

	// Load global config for scaffold state (best-effort; nil is safe).
	globalPath, _ := profileconfig.GlobalPath()
	globalCfg, _ := profileconfig.LoadGlobal(globalPath)

	// Pre-fill: replace "TODO" with known templates from central state.
	types = scaffold.PreFill(types, globalCfg, store.StatePath, store.Load)

	// Determine writer.
	var writer *os.File
	if scaffoldOutputFlag != "" {
		writer, err = os.Create(scaffoldOutputFlag)
		if err != nil {
			return fmt.Errorf("state scaffold: open output file: %w", err)
		}
		defer writer.Close()
	} else {
		writer = os.Stdout
	}

	if err := scaffold.RenderYAML(writer, types); err != nil {
		return err
	}

	if scaffoldSaveFlag {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("state scaffold: get cwd: %w", err)
		}
		if err := store.SaveTypes(types, globalCfg, cwd, os.Stdin, os.Stderr); err != nil {
			return fmt.Errorf("state scaffold: save: %w", err)
		}
		output.Success("Scaffold state saved")
	}

	return nil
}
