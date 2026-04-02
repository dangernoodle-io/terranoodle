package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"dangernoodle.io/terranoodle/internal/output"
	"dangernoodle.io/terranoodle/internal/state/config"
	"dangernoodle.io/terranoodle/internal/state/importer"
	"dangernoodle.io/terranoodle/internal/state/plan"
	"dangernoodle.io/terranoodle/internal/state/prompt"
	"dangernoodle.io/terranoodle/internal/state/resolver"
	"dangernoodle.io/terranoodle/internal/ui"
)

// state import flags.
var (
	importImportFlag bool
	importMvFlag     bool
	importApplyFlag  bool
	importConfigFlag string
	importDirFlag    string
	importVarFlags   []string
	importOutputFlag string
	importPlanFlag   string
	importForceFlag  bool
)

var terraformImportFn = func(ctx context.Context, workDir, addr, id string, useTerragrunt bool) error {
	if useTerragrunt {
		return importer.TerragruntImport(ctx, workDir, addr, id)
	}
	return importer.TerraformImport(ctx, workDir, addr, id)
}

var stateImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Generate import blocks from a terraform plan",
	RunE:  runStateImport,
}

func init() {
	stateImportCmd.Flags().BoolVar(&importImportFlag, "import", false, "Generate import {} blocks")
	stateImportCmd.Flags().BoolVar(&importMvFlag, "mv", false, "Execute terraform/terragrunt import commands")
	stateImportCmd.Flags().BoolVar(&importApplyFlag, "apply", false, "Execute the operation (default: preview to stdout)")
	stateImportCmd.Flags().StringVarP(&importConfigFlag, "config", "c", "", "Path to import config file (required)")
	stateImportCmd.Flags().StringVar(&importDirFlag, "dir", "", "Working directory (default: current directory)")
	stateImportCmd.Flags().StringArrayVar(&importVarFlags, "var", nil, "Variable override in key=value form (repeatable)")
	stateImportCmd.Flags().StringVarP(&importOutputFlag, "output", "o", "", "Output file path (default: imports.tf)")
	stateImportCmd.Flags().StringVar(&importPlanFlag, "plan", "", "Path to existing plan JSON (optional)")
	stateImportCmd.Flags().BoolVar(&importForceFlag, "force", false, "Overwrite existing output file")
	_ = stateImportCmd.MarkFlagRequired("config")

	stateCmd.AddCommand(stateImportCmd)
}

func runStateImport(cmd *cobra.Command, args []string) error {
	if !importImportFlag && !importMvFlag {
		return fmt.Errorf("state import: one of --import or --mv is required")
	}
	if importImportFlag && importMvFlag {
		return fmt.Errorf("state import: --import and --mv are mutually exclusive")
	}

	ctx := context.Background()

	// Parse --var flags into a map.
	varOverrides, err := parseVarFlags(importVarFlags)
	if err != nil {
		return err
	}

	// Load and validate config.
	cfg, err := config.Load(importConfigFlag)
	if err != nil {
		return err
	}
	if err := config.Validate(cfg); err != nil {
		return err
	}

	// Resolve working environment.
	env, err := resolveWorkEnv(ctx, importDirFlag, true)
	if err != nil {
		return fmt.Errorf("state import: %w", err)
	}

	// Generate or load plan.
	planJSON, err := loadOrGeneratePlan(ctx, importPlanFlag, env)
	if err != nil {
		if importPlanFlag != "" {
			return fmt.Errorf("state import: read plan: %w", err)
		}
		return err
	}

	p, err := plan.Parse(bytes.NewReader(planJSON))
	if err != nil {
		return fmt.Errorf("state import: parse plan: %w", err)
	}

	creates := plan.FilterCreates(p)
	if len(creates) == 0 {
		output.Info("No resources to import")
		return nil
	}

	// Collect addresses for state check.
	addrs := make([]string, len(creates))
	for i, c := range creates {
		addrs[i] = c.Address
	}

	// Check which are already managed.
	managed, err := checkStateFn(ctx, env.workDir, addrs, env.useTerragrunt)
	if err != nil {
		return err
	}

	// Filter out already-managed resources.
	creates = filterManaged(creates, managed)

	if len(creates) == 0 {
		output.Info("No resources to import")
		return nil
	}

	// Build API client if needed for resolvers.
	var getter resolver.Getter
	if cfg.API != nil && len(cfg.Resolvers) > 0 {
		token := os.Getenv(cfg.API.TokenEnv)
		getter = resolver.NewAPIClient(cfg.API.BaseURL, token)
	}

	// Resolve IDs via config.
	result, err := resolver.Resolve(creates, cfg, varOverrides, getter)
	if err != nil {
		return err
	}

	// Collect all import entries.
	allEntries := result.Matched

	// Collect resolver names for the prompt.
	resolverNames := make([]string, 0, len(cfg.Resolvers))
	for name := range cfg.Resolvers {
		resolverNames = append(resolverNames, name)
	}

	// Merge vars for prompts.
	mergedVars := make(map[string]string, len(cfg.Vars)+len(varOverrides))
	for k, v := range cfg.Vars {
		mergedVars[k] = v
	}
	for k, v := range varOverrides {
		mergedVars[k] = v
	}

	// Handle unmatched resources interactively.
	for _, addr := range result.Unmatched {
		var resourceType string
		fields := map[string]string{}
		for _, c := range creates {
			if c.Address == addr {
				resourceType = c.Type
				if c.Change != nil {
					fields = extractFields(c.Change.After)
				}
				break
			}
		}

		var id string
		var save bool
		var resolverDef *prompt.ResolverResult

		if getter != nil {
			id, save, resolverDef, err = prompt.APIAssisted(os.Stdin, os.Stdout, addr, resourceType, fields, getter, cfg.API.BaseURL, resolverNames, mergedVars)
		} else {
			id, save, err = prompt.ManualID(os.Stdin, os.Stdout, addr, resourceType, fields)
		}
		if err != nil {
			return err
		}

		if save {
			if resolverDef != nil {
				if saveErr := prompt.SaveResolverAndType(importConfigFlag, resourceType, resolverDef); saveErr != nil {
					output.Warn("could not save resolver: %v", saveErr)
				}
			} else if id != "" {
				if saveErr := prompt.SaveTypeMapping(importConfigFlag, resourceType, id); saveErr != nil {
					output.Warn("could not save type mapping: %v", saveErr)
				}
			}
		}

		if id != "" {
			allEntries = append(allEntries, resolver.ImportEntry{
				Address: addr,
				ID:      id,
				Type:    resourceType,
			})
		}
	}

	// --- Mode-specific execution ---

	if importImportFlag {
		data := importer.GenerateImportsFile(allEntries)

		if !importApplyFlag {
			fmt.Print(string(data))
			return nil
		}

		path, err := importer.WriteImportsFile(env.dir, importOutputFlag, data, importForceFlag)
		if err != nil {
			return err
		}
		output.Success("Written: %s", path)

		stopApply := ui.Spinner("Applying imports...")
		err = applyFn(ctx, env.workDir, env.useTerragrunt)
		stopApply()
		if err != nil {
			return err
		}

		if removeErr := importer.RemoveImportsFile(path); removeErr != nil {
			output.Warn("could not remove imports file: %v", removeErr)
		}

		output.Success("Import complete")
		return nil
	}

	// --mv mode
	if !importApplyFlag {
		for _, e := range allEntries {
			output.DryRun("%s import %s %s", binaryName(env.useTerragrunt), output.Cyan("%s", e.Address), output.Cyan("%s", e.ID))
		}
		return nil
	}

	// --mv --apply: execute terraform import for each entry
	importDir := env.workDir
	if env.useTerragrunt {
		importDir = env.dir
	}
	for _, e := range allEntries {
		stop := ui.Spinner(fmt.Sprintf("Importing %s", e.Address))
		err = terraformImportFn(ctx, importDir, e.Address, e.ID, env.useTerragrunt)
		stop()
		if err != nil {
			return err
		}
		output.Item("%s", e.Address)
	}
	output.Success("Import complete")
	return nil
}
