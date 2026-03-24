package plan

import (
	"encoding/json"
	"io"

	tfjson "github.com/hashicorp/terraform-json"
)

// Parse decodes a Terraform plan JSON from r into a typed Plan struct.
func Parse(r io.Reader) (*tfjson.Plan, error) {
	var p tfjson.Plan
	if err := json.NewDecoder(r).Decode(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

// FilterCreates returns only the resource changes whose action is exactly
// [create] (i.e. net-new resources, not replacements or updates).
func FilterCreates(p *tfjson.Plan) []*tfjson.ResourceChange {
	var creates []*tfjson.ResourceChange
	for _, rc := range p.ResourceChanges {
		if rc.Change != nil && rc.Change.Actions.Create() {
			creates = append(creates, rc)
		}
	}
	return creates
}

// FilterDestroys returns only the resource changes whose action includes delete.
func FilterDestroys(p *tfjson.Plan) []*tfjson.ResourceChange {
	var destroys []*tfjson.ResourceChange
	for _, rc := range p.ResourceChanges {
		if rc.Change != nil && rc.Change.Actions.Delete() {
			destroys = append(destroys, rc)
		}
	}
	return destroys
}
