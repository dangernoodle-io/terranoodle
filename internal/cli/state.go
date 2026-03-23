package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/spf13/cobra"

	"dangernoodle.io/terratools/internal/output"
	"dangernoodle.io/terratools/internal/state/config"
	"dangernoodle.io/terratools/internal/state/importer"
	"dangernoodle.io/terratools/internal/state/plan"
	"dangernoodle.io/terratools/internal/state/prompt"
	"dangernoodle.io/terratools/internal/state/resolver"
	"dangernoodle.io/terratools/internal/state/scaffold"
	"dangernoodle.io/terratools/internal/ui"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "State management commands",
}

// state import flags.
var (
	importConfigFlag string
	importDirFlag    string
	importVarFlags   []string
	importDryRunFlag bool
	importForceFlag  bool
)

// state scaffold flags.
var (
	scaffoldDirFlag           string
	scaffoldOutputFlag        string
	scaffoldFetchRegistryFlag bool
)

// Function variables for testing seams.
var (
	generatePlanJSONFn = generatePlanJSON
	checkVersionFn     = importer.CheckVersion
	checkStateFn       = func(ctx context.Context, workDir string, addrs []string, useTerragrunt bool) ([]string, error) {
		return importer.CheckState(ctx, workDir, addrs, useTerragrunt)
	}
	checkInitFn = importer.CheckInit
	applyFn     = func(ctx context.Context, workDir string, useTerragrunt bool) error {
		if useTerragrunt {
			return importer.TerragruntApply(ctx, workDir)
		}
		return importer.Apply(ctx, workDir)
	}
)

var stateImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Generate import blocks from a terraform plan",
	RunE:  runStateImport,
}

var stateScaffoldCmd = &cobra.Command{
	Use:   "scaffold",
	Short: "Scaffold an import config from existing state",
	RunE:  runStateScaffold,
}

func init() {
	stateImportCmd.Flags().StringVarP(&importConfigFlag, "config", "c", "", "Path to import config file (required)")
	stateImportCmd.Flags().StringVar(&importDirFlag, "dir", "", "Working directory (default: current directory)")
	stateImportCmd.Flags().StringArrayVar(&importVarFlags, "var", nil, "Variable override in key=value form (repeatable)")
	stateImportCmd.Flags().BoolVar(&importDryRunFlag, "dry-run", false, "Preview imports without writing files")
	stateImportCmd.Flags().BoolVar(&importForceFlag, "force", false, "Overwrite existing imports.tf")
	_ = stateImportCmd.MarkFlagRequired("config")

	stateScaffoldCmd.Flags().StringVar(&scaffoldDirFlag, "dir", "", "Working directory (default: current directory)")
	stateScaffoldCmd.Flags().StringVarP(&scaffoldOutputFlag, "output", "o", "", "Output file (default: stdout)")
	stateScaffoldCmd.Flags().BoolVar(&scaffoldFetchRegistryFlag, "fetch-registry", false, "Fetch import formats from the Terraform registry")

	stateCmd.AddCommand(stateImportCmd)
	stateCmd.AddCommand(stateScaffoldCmd)
}

func resolveDir(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	return os.Getwd()
}

func detectTerragrunt(dir string) bool {
	cachePath := filepath.Join(dir, ".terragrunt-cache")
	info, err := os.Stat(cachePath)
	return err == nil && info.IsDir()
}

func generatePlanJSON(ctx context.Context, workDir string, useTerragrunt bool) ([]byte, error) {
	if useTerragrunt {
		return importer.TerragruntGeneratePlan(ctx, workDir, false)
	}
	return importer.GeneratePlan(ctx, workDir, false)
}

// parseVarFlags splits key=value strings into a map.
func parseVarFlags(vars []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, v := range vars {
		idx := strings.Index(v, "=")
		if idx < 0 {
			return nil, fmt.Errorf("state import: invalid --var %q: expected key=value", v)
		}
		result[v[:idx]] = v[idx+1:]
	}
	return result, nil
}

// filterManaged removes resources whose addresses appear in the managed list.
func filterManaged(creates []*tfjson.ResourceChange, managed []string) []*tfjson.ResourceChange {
	if len(managed) == 0 {
		return creates
	}
	managedSet := make(map[string]bool, len(managed))
	for _, a := range managed {
		managedSet[a] = true
	}
	var filtered []*tfjson.ResourceChange
	for _, c := range creates {
		if !managedSet[c.Address] {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// extractFields extracts string-representable fields from a plan resource's Change.After.
func extractFields(after interface{}) map[string]string {
	fields := make(map[string]string)
	m, ok := after.(map[string]interface{})
	if !ok {
		return fields
	}
	for k, v := range m {
		switch val := v.(type) {
		case string:
			fields[k] = val
		case bool:
			fields[k] = fmt.Sprintf("%v", val)
		case float64:
			fields[k] = fmt.Sprintf("%g", val)
		}
	}
	return fields
}

func runStateImport(cmd *cobra.Command, args []string) error {
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

	// Resolve working dir.
	dir, err := resolveDir(importDirFlag)
	if err != nil {
		return fmt.Errorf("state import: resolve dir: %w", err)
	}

	useTerragrunt := detectTerragrunt(dir)

	var workDir string
	if useTerragrunt {
		workDir, err = importer.FindTerragruntCache(dir)
		if err != nil {
			return err
		}
	} else {
		workDir = dir
	}

	// Check terraform version.
	if err := checkVersionFn(ctx, workDir); err != nil {
		return err
	}

	// Check init.
	if useTerragrunt {
		// For terragrunt, init is implied by having a .terragrunt-cache with .terraform/.
		// FindTerragruntCache already verifies the .terraform dir exists.
	} else {
		if err := checkInitFn(workDir); err != nil {
			return err
		}
	}

	// Generate plan.
	var planJSON []byte
	stopSpinner := ui.Spinner("Generating plan...")
	planJSON, err = generatePlanJSONFn(ctx, workDir, useTerragrunt)
	stopSpinner()
	if err != nil {
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
	managed, err := checkStateFn(ctx, workDir, addrs, useTerragrunt)
	if err != nil {
		return err
	}

	// Filter out already-managed resources.
	creates = filterManaged(creates, managed)

	if len(creates) == 0 {
		output.Info("No resources to import")
		return nil
	}

	// Resolve IDs via config.
	result, err := resolver.Resolve(creates, cfg, varOverrides)
	if err != nil {
		return err
	}

	// Collect all import entries.
	allEntries := result.Matched

	// Build API client if needed.
	var apiClient *resolver.APIClient
	if cfg.API != nil {
		token := os.Getenv(cfg.API.TokenEnv)
		apiClient = resolver.NewAPIClient(cfg.API.BaseURL, token)
	}

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
		// Find the resource change for this address.
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

		if cfg.API != nil && apiClient != nil {
			id, save, resolverDef, err = prompt.APIAssisted(os.Stdin, os.Stdout, addr, resourceType, fields, apiClient, cfg.API.BaseURL, resolverNames, mergedVars)
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

	// Generate imports file content.
	data := importer.GenerateImportsFile(allEntries)

	if importDryRunFlag {
		fmt.Print(string(data))
		return nil
	}

	path, err := importer.WriteImportsFile(workDir, data, importForceFlag)
	if err != nil {
		return err
	}
	output.Info("Written: %s", path)

	// Apply imports.
	stopApply := ui.Spinner("Applying imports...")
	err = applyFn(ctx, workDir, useTerragrunt)
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

func runStateScaffold(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Resolve working dir.
	dir, err := resolveDir(scaffoldDirFlag)
	if err != nil {
		return fmt.Errorf("state scaffold: resolve dir: %w", err)
	}

	useTerragrunt := detectTerragrunt(dir)

	var workDir string
	if useTerragrunt {
		workDir, err = importer.FindTerragruntCache(dir)
		if err != nil {
			return err
		}
	} else {
		workDir = dir
	}

	// Generate plan.
	var planJSON []byte
	stopSpinner := ui.Spinner("Generating plan...")
	planJSON, err = generatePlanJSONFn(ctx, workDir, useTerragrunt)
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

	return scaffold.RenderYAML(writer, types)
}
