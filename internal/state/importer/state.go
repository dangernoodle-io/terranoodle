package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	tfjson "github.com/hashicorp/terraform-json"
)

// CheckState runs terraform (or terragrunt when useTerragrunt is true) show -json
// and returns any of the given addresses that are already present in the state.
func CheckState(ctx context.Context, workDir string, addresses []string, useTerragrunt bool) (alreadyManaged []string, err error) {
	var bin string
	if useTerragrunt {
		bin, err = tgBinary()
	} else {
		bin, err = tfBinary()
	}
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin, "show", "-json")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("importer: terraform show: %w", err)
	}
	var state tfjson.State
	if err := json.Unmarshal(out, &state); err != nil {
		return nil, fmt.Errorf("importer: parse state: %w", err)
	}
	if state.Values == nil {
		return nil, nil
	}

	managed := collectAddresses(state.Values.RootModule)

	want := make(map[string]struct{}, len(addresses))
	for _, a := range addresses {
		want[a] = struct{}{}
	}

	for _, a := range managed {
		if _, ok := want[a]; ok {
			alreadyManaged = append(alreadyManaged, a)
		}
	}
	return alreadyManaged, nil
}

func collectAddresses(mod *tfjson.StateModule) []string {
	if mod == nil {
		return nil
	}
	var out []string
	for _, r := range mod.Resources {
		out = append(out, r.Address)
	}
	for _, child := range mod.ChildModules {
		out = append(out, collectAddresses(child)...)
	}
	return out
}
