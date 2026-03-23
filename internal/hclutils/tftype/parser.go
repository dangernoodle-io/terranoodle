package tftype

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/typeexpr"
	"github.com/zclconf/go-cty/cty"
)

// ParseConstraint parses a variable's type expression into a cty.Type.
// Returns cty.DynamicPseudoType for nil expressions (no constraint = any).
func ParseConstraint(expr hcl.Expression) (cty.Type, error) {
	if expr == nil {
		return cty.DynamicPseudoType, nil
	}

	ty, _, diags := typeexpr.TypeConstraintWithDefaults(expr)
	if diags.HasErrors() {
		return cty.NilType, fmt.Errorf("parsing type constraint: %s", diags.Error())
	}

	return ty, nil
}
