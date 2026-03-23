package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/spf13/cobra"

	"dangernoodle.io/terra-tools/internal/output"
	"dangernoodle.io/terra-tools/internal/state/config"
	"dangernoodle.io/terra-tools/internal/state/importer"
	"dangernoodle.io/terra-tools/internal/state/plan"
	"dangernoodle.io/terra-tools/internal/state/prompt"
	"dangernoodle.io/terra-tools/internal/state/resolver"
	"dangernoodle.io/terra-tools/internal/state/scaffold"
	"dangernoodle.io/terra-tools/internal/ui"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "State import operations",
}

// --- import subcommand ---

var (
	stateImportConfigFiles    []string
	stateImportPlanFile       string
	stateImportVars           []string
	stateImportNonInteractive bool
	stateImportKeep           bool
	stateImportForce          bool
	stateImportTerragrunt     bool
	stateImportNoTerragrunt   bool
	stateImportSkipStateCheck bool
	stateImportDryRun         bool
	stateImportVerbose        bool
)

var (
	stateScaffoldVerbose bool
)

var stateImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Generate import blocks from terraform plan and apply (use --dry-run for preview)",
	RunE:  runStateImport,
}

func init() {
	stateImportCmd.Flags().StringArrayVarP(&stateImportConfigFiles, "config", "c", nil, "mapping config YAML file (repeatable)")
	stateImportCmd.Flags().StringVarP(&stateImportPlanFile, "plan", "p", "", "path to plan JSON")
	stateImportCmd.Flags().StringArrayVar(&stateImportVars, "var", nil, "key=val variable override (repeatable)")
	stateImportCmd.Flags().BoolVar(&stateImportNonInteractive, "non-interactive", false, "skip unmatched resources instead of prompting")
	stateImportCmd.Flags().BoolVar(&stateImportKeep, "keep", false, "keep generated import files after execution")
	stateImportCmd.Flags().BoolVar(&stateImportForce, "force", false, "force import even if resource already exists in state")
	stateImportCmd.Flags().BoolVar(&stateImportTerragrunt, "terragrunt", false, "invoke terragrunt instead of terraform")
	stateImportCmd.Flags().BoolVar(&stateImportNoTerragrunt, "no-terragrunt", false, "force use of terraform even if terragrunt is detected")
	stateImportCmd.Flags().BoolVar(&stateImportSkipStateCheck, "skip-state-check", false, "skip pre-import state existence check")
	stateImportCmd.Flags().BoolVar(&stateImportDryRun, "dry-run", false, "resolve import IDs and print HCL import blocks without applying")
	stateImportCmd.Flags().BoolVar(&stateImportVerbose, "verbose", false, "show full terraform/terragrunt output during plan generation")

	stateCmd.AddCommand(stateImportCmd)
}

func runStateImport(_ *cobra.Command, _ []string) error {
	if stateImportDryRun {
		return runStateImportDryRun()
	}
	return runStateImportRun()
}

func runStateImportRun() error {
	ctx := context.Background()

	// 1. Parse --var overrides; validate format eagerly.
	varOverrides, err := stateParseVarOverrides(stateImportVars)
	if err != nil {
		return err
	}

	// 2. Load and merge all config files.
	cfg, err := stateLoadConfigs(stateImportConfigFiles)
	if err != nil {
		return err
	}

	// 3. Determine working directory and whether to use Terragrunt.
	workDir := "."
	useTerragrunt := stateImportTerragrunt
	if !stateImportNoTerragrunt && !useTerragrunt {
		if _, err := os.Stat("terragrunt.hcl"); err == nil {
			useTerragrunt = true
		}
	}

	// 4. Obtain plan JSON (from file or auto-generated), filtering to creates.
	creates, err := stateObtainPlan(stateImportPlanFile, useTerragrunt, stateImportVerbose)
	if err != nil {
		return err
	}

	// 5. Resolve IDs.
	resolved, err := resolver.Resolve(creates, cfg, varOverrides)
	if err != nil {
		return err
	}

	// 6. Interactive prompts for unmatched resources.
	if len(resolved.Unmatched) > 0 && !stateImportNonInteractive {
		// Build a quick lookup: address → *ResourceChange.
		rcByAddr := make(map[string]*tfjson.ResourceChange, len(creates))
		for _, rc := range creates {
			rcByAddr[rc.Address] = rc
		}

		// Lazily create the API client only when there are unmatched resources
		// and an API block is configured.
		var apiClient *resolver.APIClient
		var baseURL string
		if cfg.API != nil {
			token := os.Getenv(cfg.API.TokenEnv)
			apiClient = resolver.NewAPIClient(cfg.API.BaseURL, token)
			baseURL = cfg.API.BaseURL
		}

		// Collect names of existing resolvers for the dependency prompt.
		existingResolvers := make([]string, 0, len(cfg.Resolvers))
		for name := range cfg.Resolvers {
			existingResolvers = append(existingResolvers, name)
		}

		// Merge vars for template rendering context.
		mergedVars := make(map[string]string, len(cfg.Vars)+len(varOverrides))
		for k, v := range cfg.Vars {
			mergedVars[k] = v
		}
		for k, v := range varOverrides {
			mergedVars[k] = v
		}

		for _, addr := range resolved.Unmatched {
			rc, ok := rcByAddr[addr]
			if !ok {
				continue
			}
			fields := stateExtractFields(rc)

			var id string
			var save bool
			var promptErr error

			if cfg.API != nil {
				var resolverDef *prompt.ResolverResult
				id, save, resolverDef, promptErr = prompt.APIAssisted(
					os.Stdin, os.Stdout,
					addr, rc.Type, fields,
					apiClient, baseURL,
					existingResolvers,
					mergedVars,
				)
				if promptErr != nil {
					return promptErr
				}
				if resolverDef != nil && save {
					if err := prompt.SaveResolverAndType(stateImportConfigFiles[0], rc.Type, resolverDef); err != nil {
						output.Warn("warning: could not save resolver: %v", err)
					}
				}
			} else {
				id, save, promptErr = prompt.ManualID(os.Stdin, os.Stdout, addr, rc.Type, fields)
				if promptErr != nil {
					return promptErr
				}
				if save {
					if err := prompt.SaveTypeMapping(stateImportConfigFiles[0], rc.Type, id); err != nil {
						output.Warn("warning: could not save template: %v", err)
					}
				}
			}

			if id == "" {
				continue
			}

			resolved.Matched = append(resolved.Matched, resolver.ImportEntry{Address: addr, ID: id, Type: rc.Type})
		}
	}

	// 7. If no matches, bail early.
	if len(resolved.Matched) == 0 {
		fmt.Println("no resources to import")
		return nil
	}

	matched := resolved.Matched

	// 9. State check: filter out resources already managed by Terraform.
	if !stateImportSkipStateCheck {
		addresses := make([]string, len(matched))
		for i, entry := range matched {
			addresses[i] = entry.Address
		}

		alreadyManaged, err := importer.CheckState(ctx, workDir, addresses, useTerragrunt)
		if err != nil {
			return err
		}

		managedSet := make(map[string]struct{}, len(alreadyManaged))
		for _, addr := range alreadyManaged {
			managedSet[addr] = struct{}{}
		}

		filtered := matched[:0]
		for _, entry := range matched {
			if _, exists := managedSet[entry.Address]; exists {
				fmt.Printf("notice: skipping %s — already managed in state\n", entry.Address)
				continue
			}
			filtered = append(filtered, entry)
		}
		matched = filtered

		if len(matched) == 0 {
			fmt.Println("no resources to import")
			return nil
		}
	}

	// 10. Version check — must be >= 1.5 for native import blocks.
	// 11. Init check — working directory must have been initialised.
	if !useTerragrunt {
		if err := importer.CheckVersion(ctx, workDir); err != nil {
			return err
		}
		if err := importer.CheckInit(workDir); err != nil {
			return err
		}
	}

	// 12. Generate imports file content.
	data := importer.GenerateImportsFile(matched)

	// 13. Write imports file.
	importsPath, err := importer.WriteImportsFile(workDir, data, stateImportForce)
	if err != nil {
		return err
	}

	// 14. Run terraform/terragrunt apply.
	if useTerragrunt {
		if err := importer.TerragruntApply(ctx, workDir); err != nil {
			return err
		}
	} else {
		if err := importer.Apply(ctx, workDir); err != nil {
			return err
		}
	}

	// 15. Clean up imports file unless --keep is set.
	if !stateImportKeep {
		if err := importer.RemoveImportsFile(importsPath); err != nil {
			return fmt.Errorf("importer: remove imports file: %w", err)
		}
	}

	// 16. Print summary.
	output.Success("%d resource(s) imported successfully", len(matched))

	// 17. Warn about still-unmatched resources (those not resolved interactively).
	if len(resolved.Unmatched) > 0 {
		resolvedAddrs := make(map[string]struct{}, len(resolved.Matched))
		for _, m := range resolved.Matched {
			resolvedAddrs[m.Address] = struct{}{}
		}
		var stillUnmatched []string
		for _, addr := range resolved.Unmatched {
			if _, ok := resolvedAddrs[addr]; !ok {
				stillUnmatched = append(stillUnmatched, addr)
			}
		}
		if len(stillUnmatched) > 0 {
			output.Warn("warning: the following resources had no type mapping and were skipped:")
			for _, addr := range stillUnmatched {
				fmt.Printf("  - %s\n", addr)
			}
		}
	}

	return nil
}

func runStateImportDryRun() error {
	// 1. Load and merge all config files.
	cfg, err := stateLoadConfigs(stateImportConfigFiles)
	if err != nil {
		return err
	}

	// Parse --var overrides; validate format eagerly.
	varOverrides, err := stateParseVarOverrides(stateImportVars)
	if err != nil {
		return err
	}

	// 2. Determine whether to use Terragrunt.
	useTerragrunt := stateImportTerragrunt
	if !stateImportNoTerragrunt && !useTerragrunt {
		if _, err := os.Stat("terragrunt.hcl"); err == nil {
			useTerragrunt = true
		}
	}

	// 3. Obtain plan JSON (from file or auto-generated), filtering to creates.
	creates, err := stateObtainPlan(stateImportPlanFile, useTerragrunt, stateImportVerbose)
	if err != nil {
		return err
	}

	resolved, err := resolver.Resolve(creates, cfg, varOverrides)
	if err != nil {
		return err
	}

	// 5. Interactive prompts for unmatched resources.
	extra := resolved.Matched // will grow as user provides IDs
	if len(resolved.Unmatched) > 0 && !stateImportNonInteractive {
		// Build a quick lookup: address → *ResourceChange.
		rcByAddr := make(map[string]*tfjson.ResourceChange, len(creates))
		for _, rc := range creates {
			rcByAddr[rc.Address] = rc
		}

		// Lazily create the API client only when there are unmatched resources
		// and an API block is configured.
		var apiClient *resolver.APIClient
		var baseURL string
		if cfg.API != nil {
			token := os.Getenv(cfg.API.TokenEnv)
			apiClient = resolver.NewAPIClient(cfg.API.BaseURL, token)
			baseURL = cfg.API.BaseURL
		}

		// Collect names of existing resolvers for the dependency prompt.
		existingResolvers := make([]string, 0, len(cfg.Resolvers))
		for name := range cfg.Resolvers {
			existingResolvers = append(existingResolvers, name)
		}

		// Merge vars for template rendering context.
		mergedVars := make(map[string]string, len(cfg.Vars)+len(varOverrides))
		for k, v := range cfg.Vars {
			mergedVars[k] = v
		}
		for k, v := range varOverrides {
			mergedVars[k] = v
		}

		for _, addr := range resolved.Unmatched {
			rc, ok := rcByAddr[addr]
			if !ok {
				continue
			}
			fields := stateExtractFields(rc)

			var id string
			var save bool
			var promptErr error

			if cfg.API != nil {
				var resolverDef *prompt.ResolverResult
				id, save, resolverDef, promptErr = prompt.APIAssisted(
					os.Stdin, os.Stdout,
					addr, rc.Type, fields,
					apiClient, baseURL,
					existingResolvers,
					mergedVars,
				)
				if promptErr != nil {
					return promptErr
				}
				if resolverDef != nil && save {
					if err := prompt.SaveResolverAndType(stateImportConfigFiles[0], rc.Type, resolverDef); err != nil {
						output.Warn("warning: could not save resolver: %v", err)
					}
				}
			} else {
				id, save, promptErr = prompt.ManualID(os.Stdin, os.Stdout, addr, rc.Type, fields)
				if promptErr != nil {
					return promptErr
				}
				if save {
					if err := prompt.SaveTypeMapping(stateImportConfigFiles[0], rc.Type, id); err != nil {
						output.Warn("warning: could not save template: %v", err)
					}
				}
			}

			if id == "" {
				continue
			}

			extra = append(extra, resolver.ImportEntry{Address: addr, ID: id, Type: rc.Type})
		}
	}

	// 6. Print HCL import blocks for matched resources.
	for _, m := range extra {
		fmt.Printf("import {\n  to = %s\n  id = %q\n}\n\n", m.Address, m.ID)
	}

	// 7. Print remaining unmatched resources as comments (non-interactive or skipped).
	if len(resolved.Unmatched) > 0 {
		// Determine which addresses were not resolved interactively.
		resolvedAddrs := make(map[string]struct{}, len(extra))
		for _, m := range extra {
			resolvedAddrs[m.Address] = struct{}{}
		}
		var stillUnmatched []string
		for _, addr := range resolved.Unmatched {
			if _, ok := resolvedAddrs[addr]; !ok {
				stillUnmatched = append(stillUnmatched, addr)
			}
		}
		if len(stillUnmatched) > 0 {
			fmt.Println("# Unmatched resources (no type mapping found):")
			for _, addr := range stillUnmatched {
				fmt.Printf("# - %s\n", addr)
			}
			fmt.Println()
		}
	}

	// 8. Print summary.
	output.Info("%d resource(s) to import, %d unmatched", len(extra), len(resolved.Unmatched)-(len(extra)-len(resolved.Matched)))

	return nil
}

// --- scaffold subcommand ---

var (
	stateScaffoldPlanFile     string
	stateScaffoldOutputFile   string
	stateScaffoldSkipRegistry bool
)

var stateScaffoldCmd = &cobra.Command{
	Use:   "scaffold",
	Short: "Generate a starter YAML config skeleton from a plan JSON",
	RunE:  runStateScaffold,
}

func init() {
	stateScaffoldCmd.Flags().StringVarP(&stateScaffoldPlanFile, "plan", "p", "", "path to plan JSON")
	stateScaffoldCmd.Flags().StringVarP(&stateScaffoldOutputFile, "output", "o", "", "output file path (default: stdout)")
	stateScaffoldCmd.Flags().BoolVar(&stateScaffoldSkipRegistry, "skip-registry", false, "skip fetching import formats from the Terraform registry")
	stateScaffoldCmd.Flags().BoolVar(&stateScaffoldVerbose, "verbose", false, "show full terraform/terragrunt output during plan generation")

	stateCmd.AddCommand(stateScaffoldCmd)
}

func runStateScaffold(_ *cobra.Command, _ []string) error {
	// 1. Detect whether to use Terragrunt (auto-detect only; no flags for scaffold).
	useTerragrunt := false
	if _, err := os.Stat("terragrunt.hcl"); err == nil {
		useTerragrunt = true
	}

	// 2. Obtain plan JSON (from file or auto-generated), filtering to creates.
	creates, err := stateObtainPlan(stateScaffoldPlanFile, useTerragrunt, stateScaffoldVerbose)
	if err != nil {
		return err
	}

	// 2. Optionally fetch import formats from the provider registry.
	var formats map[string]string
	if !stateScaffoldSkipRegistry {
		// Collect unique resource types.
		seen := make(map[string]struct{})
		for _, rc := range creates {
			if rc != nil {
				seen[rc.Type] = struct{}{}
			}
		}

		cache := make(map[string]string)
		formats = make(map[string]string, len(seen))
		ctx := context.Background()
		for rt := range seen {
			formats[rt] = scaffold.FetchImportFormat(ctx, rt, cache)
		}
	}

	// 3. Generate scaffold type info.
	types := scaffold.Generate(creates, formats)

	// 4. Write output.
	if stateScaffoldOutputFile != "" {
		out, err := os.Create(stateScaffoldOutputFile)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer out.Close()

		if err := scaffold.RenderYAML(out, types); err != nil {
			return fmt.Errorf("render yaml: %w", err)
		}

		output.Success("scaffold config written to %s", stateScaffoldOutputFile)
		return nil
	}

	return scaffold.RenderYAML(os.Stdout, types)
}

// --- shared helpers ---

// stateLoadConfigs loads and merges one or more YAML config files, then
// validates the result. It returns an error if paths is empty.
func stateLoadConfigs(paths []string) (*config.Config, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one --config file is required")
	}

	cfgs := make([]*config.Config, 0, len(paths))
	for _, path := range paths {
		c, err := config.Load(path)
		if err != nil {
			return nil, err
		}
		cfgs = append(cfgs, c)
	}

	cfg := config.Merge(cfgs...)
	if err := config.Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// stateParseVarOverrides parses a slice of "key=val" strings into a map. It
// returns an error if any entry is not in that format.
func stateParseVarOverrides(vars []string) (map[string]string, error) {
	overrides := make(map[string]string, len(vars))
	for _, kv := range vars {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("--var %q is not in key=val format", kv)
		}
		overrides[k] = v
	}
	return overrides, nil
}

// stateObtainPlan returns the resource changes that are creates. If path is
// non-empty the plan is read from that file (emitting a stale warning when
// older than one hour). Otherwise a plan is generated on the fly using
// terraform or terragrunt.
func stateObtainPlan(path string, useTerragrunt bool, verbose bool) ([]*tfjson.ResourceChange, error) {
	if path != "" {
		// Use provided plan file (existing behavior)
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open plan: %w", err)
		}
		defer f.Close()

		if fi, err := f.Stat(); err == nil {
			if age := time.Since(fi.ModTime()); age > time.Hour {
				output.Warn("warning: plan file %q is %.0f minutes old — it may be stale", path, age.Minutes())
			}
		}

		p, err := plan.Parse(f)
		if err != nil {
			return nil, fmt.Errorf("parse plan: %w", err)
		}
		return plan.FilterCreates(p), nil
	}

	// Auto-generate plan
	ctx := context.Background()
	var stop func()
	if !verbose {
		stop = ui.Spinner("Generating plan...")
	} else {
		stop = func() {} // no-op
	}

	var jsonBytes []byte
	var err error
	if useTerragrunt {
		jsonBytes, err = importer.TerragruntGeneratePlan(ctx, ".", verbose)
	} else {
		jsonBytes, err = importer.GeneratePlan(ctx, ".", verbose)
	}
	stop()

	if err != nil {
		return nil, err
	}

	p, err := plan.Parse(bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("parse generated plan: %w", err)
	}
	return plan.FilterCreates(p), nil
}

// stateExtractFields converts a ResourceChange's After map to a flat string
// map for display in prompts. Sensitive values are masked as "(sensitive)".
func stateExtractFields(rc *tfjson.ResourceChange) map[string]string {
	if rc.Change == nil || rc.Change.After == nil {
		return nil
	}
	after, ok := rc.Change.After.(map[string]interface{})
	if !ok {
		return nil
	}

	sensitiveKeys := make(map[string]bool)
	if rc.Change.AfterSensitive != nil {
		if sens, ok := rc.Change.AfterSensitive.(map[string]interface{}); ok {
			for k, v := range sens {
				if b, ok := v.(bool); ok && b {
					sensitiveKeys[k] = true
				}
			}
		}
	}

	fields := make(map[string]string, len(after))
	for k, v := range after {
		if v == nil {
			continue
		}
		if sensitiveKeys[k] {
			fields[k] = "(sensitive)"
		} else {
			fields[k] = fmt.Sprintf("%v", v)
		}
	}
	return fields
}
