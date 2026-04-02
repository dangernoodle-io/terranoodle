package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/spf13/cobra"

	"dangernoodle.io/terranoodle/internal/state/importer"
	"dangernoodle.io/terranoodle/internal/ui"
	"dangernoodle.io/terranoodle/internal/version"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "State management commands",
}

// Function variables for testing seams (shared across subcommands).
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
)

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
