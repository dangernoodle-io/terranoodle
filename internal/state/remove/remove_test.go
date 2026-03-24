package remove

import (
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
)

func TestDetectFromPlan(t *testing.T) {
	tests := []struct {
		name     string
		plan     *tfjson.Plan
		expected []string
	}{
		{
			name: "only destroys",
			plan: &tfjson.Plan{
				ResourceChanges: []*tfjson.ResourceChange{
					{
						Address: "acme_resource.example",
						Type:    "acme_resource",
						Change: &tfjson.Change{
							Actions: tfjson.Actions{tfjson.ActionDelete},
						},
					},
					{
						Address: "acme_widget.item",
						Type:    "acme_widget",
						Change: &tfjson.Change{
							Actions: tfjson.Actions{tfjson.ActionDelete},
						},
					},
				},
			},
			expected: []string{"acme_resource.example", "acme_widget.item"},
		},
		{
			name: "destroys with PreviousAddress excluded",
			plan: &tfjson.Plan{
				ResourceChanges: []*tfjson.ResourceChange{
					{
						Address:         "acme_resource.new",
						PreviousAddress: "acme_resource.old",
						Type:            "acme_resource",
						Change: &tfjson.Change{
							Actions: tfjson.Actions{tfjson.ActionDelete},
						},
					},
					{
						Address: "acme_widget.item",
						Type:    "acme_widget",
						Change: &tfjson.Change{
							Actions: tfjson.Actions{tfjson.ActionDelete},
						},
					},
				},
			},
			expected: []string{"acme_widget.item"},
		},
		{
			name: "mixed actions only deletes",
			plan: &tfjson.Plan{
				ResourceChanges: []*tfjson.ResourceChange{
					{
						Address: "acme_resource.created",
						Type:    "acme_resource",
						Change: &tfjson.Change{
							Actions: tfjson.Actions{tfjson.ActionCreate},
						},
					},
					{
						Address: "acme_resource.updated",
						Type:    "acme_resource",
						Change: &tfjson.Change{
							Actions: tfjson.Actions{tfjson.ActionUpdate},
						},
					},
					{
						Address: "acme_resource.deleted",
						Type:    "acme_resource",
						Change: &tfjson.Change{
							Actions: tfjson.Actions{tfjson.ActionDelete},
						},
					},
				},
			},
			expected: []string{"acme_resource.deleted"},
		},
		{
			name: "empty plan",
			plan: &tfjson.Plan{
				ResourceChanges: []*tfjson.ResourceChange{},
			},
			expected: []string{},
		},
		{
			name: "nil change skipped",
			plan: &tfjson.Plan{
				ResourceChanges: []*tfjson.ResourceChange{
					{
						Address: "acme_resource.example",
						Type:    "acme_resource",
						Change:  nil,
					},
					{
						Address: "acme_widget.item",
						Type:    "acme_widget",
						Change: &tfjson.Change{
							Actions: tfjson.Actions{tfjson.ActionDelete},
						},
					},
				},
			},
			expected: []string{"acme_widget.item"},
		},
		{
			name: "sorted by address",
			plan: &tfjson.Plan{
				ResourceChanges: []*tfjson.ResourceChange{
					{
						Address: "zebra_resource.z",
						Type:    "zebra_resource",
						Change: &tfjson.Change{
							Actions: tfjson.Actions{tfjson.ActionDelete},
						},
					},
					{
						Address: "alpha_resource.a",
						Type:    "alpha_resource",
						Change: &tfjson.Change{
							Actions: tfjson.Actions{tfjson.ActionDelete},
						},
					},
				},
			},
			expected: []string{"alpha_resource.a", "zebra_resource.z"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFromPlan(tt.plan)

			if len(got) != len(tt.expected) {
				t.Errorf("DetectFromPlan returned %d targets, expected %d", len(got), len(tt.expected))
			}

			for i, target := range got {
				if i >= len(tt.expected) {
					break
				}
				if target.Address != tt.expected[i] {
					t.Errorf("target %d: got %q, expected %q", i, target.Address, tt.expected[i])
				}
			}
		})
	}
}
