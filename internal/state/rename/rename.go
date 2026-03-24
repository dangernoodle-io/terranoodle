package rename

import (
	"sort"

	tfjson "github.com/hashicorp/terraform-json"
)

// RenamePair represents a resource address rename.
type RenamePair struct {
	From string
	To   string
}

// Candidate represents a destroyed resource with potential create matches of the same type.
type Candidate struct {
	Destroy *tfjson.ResourceChange
	Creates []*tfjson.ResourceChange
}

// DetectFromPlan returns all resource changes where PreviousAddress is set.
// These are definite renames — Terraform already knows the resource moved.
func DetectFromPlan(p *tfjson.Plan) []RenamePair {
	var pairs []RenamePair
	for _, rc := range p.ResourceChanges {
		if rc.PreviousAddress != "" {
			pairs = append(pairs, RenamePair{
				From: rc.PreviousAddress,
				To:   rc.Address,
			})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].From < pairs[j].From
	})
	return pairs
}

// MatchDestroyCreate finds destroy/create pairs of the same resource type as
// rename candidates. It excludes resources that already have PreviousAddress set
// (those are definite renames handled by DetectFromPlan).
func MatchDestroyCreate(p *tfjson.Plan) []Candidate {
	// Build set of addresses with PreviousAddress (already detected as renames).
	knownMoved := make(map[string]bool)
	for _, rc := range p.ResourceChanges {
		if rc.PreviousAddress != "" {
			knownMoved[rc.Address] = true
			knownMoved[rc.PreviousAddress] = true
		}
	}

	// Group destroys and creates by type.
	destroysByType := make(map[string][]*tfjson.ResourceChange)
	createsByType := make(map[string][]*tfjson.ResourceChange)

	for _, rc := range p.ResourceChanges {
		if rc.Change == nil || knownMoved[rc.Address] {
			continue
		}
		if rc.Change.Actions.Delete() {
			destroysByType[rc.Type] = append(destroysByType[rc.Type], rc)
		}
		if rc.Change.Actions.Create() {
			createsByType[rc.Type] = append(createsByType[rc.Type], rc)
		}
	}

	// Build candidates: for each destroy, find creates of the same type.
	var candidates []Candidate
	for typ, destroys := range destroysByType {
		creates, ok := createsByType[typ]
		if !ok || len(creates) == 0 {
			continue
		}
		for _, d := range destroys {
			candidates = append(candidates, Candidate{
				Destroy: d,
				Creates: creates,
			})
		}
	}

	// Sort for deterministic output.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Destroy.Address < candidates[j].Destroy.Address
	})

	return candidates
}
