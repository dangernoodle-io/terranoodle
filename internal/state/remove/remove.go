package remove

import (
	"sort"

	tfjson "github.com/hashicorp/terraform-json"
)

// RemoveTarget represents a resource to be removed from state.
type RemoveTarget struct {
	Address string
}

// DetectFromPlan returns all resource changes that are planned for deletion.
// Resources with PreviousAddress set are excluded (those are renames).
func DetectFromPlan(p *tfjson.Plan) []RemoveTarget {
	var targets []RemoveTarget
	for _, rc := range p.ResourceChanges {
		if rc.Change == nil || rc.PreviousAddress != "" {
			continue
		}
		if rc.Change.Actions.Delete() {
			targets = append(targets, RemoveTarget{Address: rc.Address})
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Address < targets[j].Address
	})
	return targets
}
