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
	terraformImportFn = func(ctx context.Context, workDir, addr, id string, useTerragrunt bool) error {
		if useTerragrunt {
			return importer.TerragruntImport(ctx, workDir, addr, id)
		}
		return importer.TerraformImport(ctx, workDir, addr, id)
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

// workEnv holds resolved working directory and terragrunt detection state.
type workEnv struct {
	dir           string
	workDir       string
	useTerragrunt bool
}

// resolveWorkEnv resolves the working directory and detects terragrunt configuration.
// If requireInit is true, checks that terraform is initialized in the terraform path.
// If requireInit is false, skips the init check (used by scaffold).
func resolveWorkEnv(ctx context.Context, dirFlag string, requireInit bool) (workEnv, error) {
	dir, err := resolveDir(dirFlag)
	if err != nil {
		return workEnv{}, err
	}

	useTerragrunt := detectTerragrunt(dir)

	if err := checkVersionFn(ctx); err != nil {
		return workEnv{}, err
	}

	var workDir string
	if useTerragrunt {
		if err := checkTerragruntVersionFn(ctx); err != nil {
			return workEnv{}, err
		}
		workDir, err = importer.FindTerragruntCache(dir)
		if err != nil {
			return workEnv{}, err
		}
	} else {
		workDir = dir
		if requireInit {
			if err := checkInitFn(workDir); err != nil {
				return workEnv{}, err
			}
		}
	}

	return workEnv{
		dir:           dir,
		workDir:       workDir,
		useTerragrunt: useTerragrunt,
	}, nil
}

// loadOrGeneratePlan loads a plan from a file or generates one.
// If planFlag is set, reads from that file. Otherwise generates a plan
// and shows a spinner. For terragrunt, plan is generated from dir;
// for terraform, from workDir.
func loadOrGeneratePlan(ctx context.Context, planFlag string, env workEnv) ([]byte, error) {
	if planFlag != "" {
		return os.ReadFile(planFlag)
	}

	planDir := env.workDir
	if env.useTerragrunt {
		planDir = env.dir
	}

	stopSpinner := ui.Spinner("Generating plan...")
	planJSON, err := generatePlanJSONFn(ctx, planDir, env.useTerragrunt)
	stopSpinner()
	return planJSON, err
}

// binaryName returns "terragrunt" if useTerragrunt is true, otherwise "terraform".
func binaryName(useTerragrunt bool) string {
	if useTerragrunt {
		return "terragrunt"
	}
	return "terraform"
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
		output.Info("Written: %s", path)

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
			fmt.Printf("%s import %s %s\n", binaryName(env.useTerragrunt), output.Bold("%s", e.Address), output.Bold("%s", e.ID))
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
	}
	output.Success("Import complete")
	return nil
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
			msg := fmt.Sprintf("%s state mv %s %s", binaryName(env.useTerragrunt), output.Bold("%s", pair.From), output.Bold("%s", pair.To))
			fmt.Println(msg)
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
	}
	output.Success("State moves complete")
	return nil
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
			fmt.Printf("%s state rm %s\n", binaryName(env.useTerragrunt), output.Bold("%s", t.Address))
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
	}
	output.Success("State removals complete")
	return nil
}
