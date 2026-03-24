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

	"dangernoodle.io/terranoodle/internal/output"
	"dangernoodle.io/terranoodle/internal/state/config"
	"dangernoodle.io/terranoodle/internal/state/importer"
	"dangernoodle.io/terranoodle/internal/state/plan"
	"dangernoodle.io/terranoodle/internal/state/prompt"
	"dangernoodle.io/terranoodle/internal/state/remove"
	"dangernoodle.io/terranoodle/internal/state/rename"
	"dangernoodle.io/terranoodle/internal/state/resolver"
	"dangernoodle.io/terranoodle/internal/state/scaffold"
	"dangernoodle.io/terranoodle/internal/ui"
	"dangernoodle.io/terranoodle/internal/version"
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

// state remove flags.
var (
	removeDirFlag   string
	removePlanFlag  string
	removeApplyFlag bool
)

// Function variables for testing seams.
var (
	generatePlanJSONFn       = generatePlanJSON
	checkVersionFn           = version.CheckTerraform
	checkTerragruntVersionFn = version.CheckTerragrunt
	checkStateFn             = func(ctx context.Context, workDir string, addrs []string, useTerragrunt bool) ([]string, error) {
		return importer.CheckState(ctx, workDir, addrs, useTerragrunt)
	}
	checkInitFn = importer.CheckInit
	applyFn     = func(ctx context.Context, workDir string, useTerragrunt bool) error {
		if useTerragrunt {
			return importer.TerragruntApply(ctx, workDir)
		}
		return importer.Apply(ctx, workDir)
	}
	stateMvFn = func(ctx context.Context, workDir, from, to string, useTerragrunt bool) error {
		if useTerragrunt {
			return rename.TerragruntStateMv(ctx, workDir, from, to)
		}
		return rename.StateMv(ctx, workDir, from, to)
	}
	stateRmFn = func(ctx context.Context, workDir, addr string, useTerragrunt bool) error {
		if useTerragrunt {
			return remove.TerragruntStateRm(ctx, workDir, addr)
		}
		return remove.StateRm(ctx, workDir, addr)
	}
	confirmCandidatesFn = func(candidates []rename.Candidate, autoConfirm bool) ([]rename.RenamePair, error) {
		return rename.ConfirmCandidates(os.Stdin, os.Stdout, candidates, autoConfirm)
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

var stateRenameCmd = &cobra.Command{
	Use:   "rename",
	Short: "Detect resource renames and generate moved blocks or execute state mv",
	RunE:  runStateRename,
}

var stateRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove destroyed resources from state without destroying infrastructure",
	RunE:  runStateRemove,
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

	stateRenameCmd.Flags().BoolVar(&renameMovedFlag, "moved", false, "Generate moved {} blocks")
	stateRenameCmd.Flags().BoolVar(&renameMvFlag, "mv", false, "Execute terraform/terragrunt state mv commands")
	stateRenameCmd.Flags().BoolVar(&renameApplyFlag, "apply", false, "Execute the operation (default: preview to stdout)")
	stateRenameCmd.Flags().StringVar(&renameDirFlag, "dir", "", "Working directory (default: current directory)")
	stateRenameCmd.Flags().StringVar(&renamePlanFlag, "plan", "", "Path to existing plan JSON (optional)")
	stateRenameCmd.Flags().StringVarP(&renameOutputFlag, "output", "o", "", "Output file path (default: moved.tf)")
	stateRenameCmd.Flags().BoolVar(&renameForceFlag, "force", false, "Overwrite existing output file")

	stateRemoveCmd.Flags().StringVar(&removeDirFlag, "dir", "", "Working directory (default: current directory)")
	stateRemoveCmd.Flags().StringVar(&removePlanFlag, "plan", "", "Path to existing plan JSON (optional)")
	stateRemoveCmd.Flags().BoolVar(&removeApplyFlag, "apply", false, "Execute state rm commands (default: preview)")

	stateCmd.AddCommand(stateImportCmd)
	stateCmd.AddCommand(stateScaffoldCmd)
	stateCmd.AddCommand(stateRenameCmd)
	stateCmd.AddCommand(stateRemoveCmd)
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

	// Check terraform version.
	if err := checkVersionFn(ctx); err != nil {
		return err
	}

	var workDir string
	if useTerragrunt {
		// Check terragrunt version.
		if err := checkTerragruntVersionFn(ctx); err != nil {
			return err
		}
		workDir, err = importer.FindTerragruntCache(dir)
		if err != nil {
			return err
		}
	} else {
		workDir = dir
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

	// Generate plan. For terragrunt, run from project dir, not cache dir.
	planDir := workDir
	if useTerragrunt {
		planDir = dir
	}
	var planJSON []byte
	stopSpinner := ui.Spinner("Generating plan...")
	planJSON, err = generatePlanJSONFn(ctx, planDir, useTerragrunt)
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

	// Check versions.
	if err := checkVersionFn(ctx); err != nil {
		return err
	}

	var workDir string
	if useTerragrunt {
		// Check terragrunt version.
		if err := checkTerragruntVersionFn(ctx); err != nil {
			return err
		}
		workDir, err = importer.FindTerragruntCache(dir)
		if err != nil {
			return err
		}
	} else {
		workDir = dir
	}

	// Generate plan. For terragrunt, run from project dir, not cache dir.
	planDir := workDir
	if useTerragrunt {
		planDir = dir
	}
	var planJSON []byte
	stopSpinner := ui.Spinner("Generating plan...")
	planJSON, err = generatePlanJSONFn(ctx, planDir, useTerragrunt)
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

func runStateRename(cmd *cobra.Command, args []string) error {
	if !renameMovedFlag && !renameMvFlag {
		return fmt.Errorf("state rename: one of --moved or --mv is required")
	}
	if renameMovedFlag && renameMvFlag {
		return fmt.Errorf("state rename: --moved and --mv are mutually exclusive")
	}

	ctx := context.Background()

	dir, err := resolveDir(renameDirFlag)
	if err != nil {
		return fmt.Errorf("state rename: resolve dir: %w", err)
	}

	useTerragrunt := detectTerragrunt(dir)

	if err := checkVersionFn(ctx); err != nil {
		return err
	}

	var workDir string
	if useTerragrunt {
		if err := checkTerragruntVersionFn(ctx); err != nil {
			return err
		}
		workDir, err = importer.FindTerragruntCache(dir)
		if err != nil {
			return err
		}
	} else {
		workDir = dir
		if err := checkInitFn(workDir); err != nil {
			return err
		}
	}

	var planJSON []byte
	if renamePlanFlag != "" {
		planJSON, err = os.ReadFile(renamePlanFlag)
		if err != nil {
			return fmt.Errorf("state rename: read plan: %w", err)
		}
	} else {
		// For terragrunt, run plan from project dir (dir), not cache dir (workDir).
		planDir := workDir
		if useTerragrunt {
			planDir = dir
		}
		stop := ui.Spinner("Generating plan...")
		planJSON, err = generatePlanJSONFn(ctx, planDir, useTerragrunt)
		stop()
		if err != nil {
			return err
		}
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
		path, err := rename.WriteMovedFile(dir, renameOutputFlag, data, renameForceFlag)
		if err != nil {
			return err
		}
		output.Success("Written: %s", path)
		return nil
	}

	// --mv mode
	if !renameApplyFlag {
		binary := "terraform"
		if useTerragrunt {
			binary = "terragrunt"
		}
		for _, pair := range pairs {
			msg := fmt.Sprintf("%s state mv %s %s", binary, output.Bold("%s", pair.From), output.Bold("%s", pair.To))
			fmt.Println(msg)
		}
		return nil
	}

	// For terragrunt, run state mv from project dir (dir), not cache dir (workDir).
	mvDir := workDir
	if useTerragrunt {
		mvDir = dir
	}
	for _, pair := range pairs {
		stop := ui.Spinner(fmt.Sprintf("Moving %s -> %s", pair.From, pair.To))
		err = stateMvFn(ctx, mvDir, pair.From, pair.To, useTerragrunt)
		stop()
		if err != nil {
			return err
		}
	}
	output.Success("State moves complete")
	return nil
}

func runStateRemove(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	dir, err := resolveDir(removeDirFlag)
	if err != nil {
		return fmt.Errorf("state remove: resolve dir: %w", err)
	}

	useTerragrunt := detectTerragrunt(dir)

	if err := checkVersionFn(ctx); err != nil {
		return err
	}

	var workDir string
	if useTerragrunt {
		if err := checkTerragruntVersionFn(ctx); err != nil {
			return err
		}
		workDir, err = importer.FindTerragruntCache(dir)
		if err != nil {
			return err
		}
	} else {
		workDir = dir
		if err := checkInitFn(workDir); err != nil {
			return err
		}
	}

	var planJSON []byte
	if removePlanFlag != "" {
		planJSON, err = os.ReadFile(removePlanFlag)
		if err != nil {
			return fmt.Errorf("state remove: read plan: %w", err)
		}
	} else {
		planDir := workDir
		if useTerragrunt {
			planDir = dir
		}
		stop := ui.Spinner("Generating plan...")
		planJSON, err = generatePlanJSONFn(ctx, planDir, useTerragrunt)
		stop()
		if err != nil {
			return err
		}
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
		binary := "terraform"
		if useTerragrunt {
			binary = "terragrunt"
		}
		for _, t := range targets {
			fmt.Printf("%s state rm %s\n", binary, output.Bold("%s", t.Address))
		}
		return nil
	}

	// Apply mode: run state rm for each target.
	rmDir := workDir
	if useTerragrunt {
		rmDir = dir
	}
	for _, t := range targets {
		stop := ui.Spinner(fmt.Sprintf("Removing %s", t.Address))
		err = stateRmFn(ctx, rmDir, t.Address, useTerragrunt)
		stop()
		if err != nil {
			return err
		}
	}
	output.Success("State removals complete")
	return nil
}
