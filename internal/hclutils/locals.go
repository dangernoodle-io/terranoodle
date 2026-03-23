package hclutils

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

// EvalLocals evaluates all attributes in a locals block body.
// It uses a multi-pass approach to handle locals that reference other locals.
// Returns a map of local name → evaluated cty.Value.
func EvalLocals(body hcl.Body, ctx *hcl.EvalContext) map[string]cty.Value {
	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		return nil
	}

	result := make(map[string]cty.Value, len(attrs))
	remaining := make(map[string]*hcl.Attribute, len(attrs))
	for name, attr := range attrs {
		remaining[name] = attr
	}

	// Ensure local.* exists in ctx for self-references
	if ctx.Variables == nil {
		ctx.Variables = map[string]cty.Value{}
	}

	// Multi-pass: keep evaluating until no more progress
	for len(remaining) > 0 {
		progress := false

		for name, attr := range remaining {
			val, diags := attr.Expr.Value(ctx)
			if diags.HasErrors() {
				continue
			}
			result[name] = val
			delete(remaining, name)
			progress = true

			// Update the local.* context for subsequent evaluations
			ctx.Variables["local"] = cty.ObjectVal(copyWithEntry(result))
		}

		if !progress {
			// Remaining locals can't be evaluated (unresolvable references).
			// Set them to DynamicVal so downstream expressions don't hard-fail.
			for name := range remaining {
				result[name] = cty.DynamicVal
			}
			ctx.Variables["local"] = cty.ObjectVal(copyWithEntry(result))
			break
		}
	}

	return result
}

// copyWithEntry creates a shallow copy of the map safe for cty.ObjectVal.
// cty.ObjectVal requires at least one entry; returns nil if empty.
func copyWithEntry(m map[string]cty.Value) map[string]cty.Value {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]cty.Value, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
