package hclutils

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// ExtractInputKeys extracts the top-level key names from an inputs expression.
// For literal object expressions like `inputs = { foo = "bar", baz = 1 }`,
// it returns a map of key name → value expression.
// For function calls like merge(), it walks the arguments.
// When ctx is non-nil, traversal expressions in merge() (e.g.,
// include.root.locals.project) are evaluated to discover their keys.
func ExtractInputKeys(expr hcl.Expression, ctx *hcl.EvalContext) (map[string]hcl.Expression, error) {
	synExpr, ok := expr.(hclsyntax.Expression)
	if !ok {
		return nil, fmt.Errorf("unexpected expression type %T", expr)
	}

	switch e := synExpr.(type) {
	case *hclsyntax.ObjectConsExpr:
		return extractObjectKeys(e)

	case *hclsyntax.FunctionCallExpr:
		if e.Name == "merge" {
			return extractMergeKeys(e, ctx)
		}
		return nil, fmt.Errorf("unsupported function in inputs: %s", e.Name)

	default:
		return nil, fmt.Errorf("unsupported inputs expression type: %T", synExpr)
	}
}

// ExtractObjectKeys extracts the top-level key→expression map from an object
// literal expression. It is exported so stack.go can reuse it for values blocks.
func ExtractObjectKeys(obj *hclsyntax.ObjectConsExpr) (map[string]hcl.Expression, error) {
	return extractObjectKeys(obj)
}

func extractObjectKeys(obj *hclsyntax.ObjectConsExpr) (map[string]hcl.Expression, error) {
	result := make(map[string]hcl.Expression)

	for _, item := range obj.Items {
		key, diags := item.KeyExpr.Value(nil)
		if diags.HasErrors() {
			// Key is a dynamic expression — skip for now
			continue
		}
		if key.Type() == cty.String {
			result[key.AsString()] = item.ValueExpr
		}
	}

	return result, nil
}

func extractMergeKeys(fn *hclsyntax.FunctionCallExpr, ctx *hcl.EvalContext) (map[string]hcl.Expression, error) {
	result := make(map[string]hcl.Expression)

	for _, arg := range fn.Args {
		switch a := arg.(type) {
		case *hclsyntax.ObjectConsExpr:
			keys, err := extractObjectKeys(a)
			if err != nil {
				return nil, err
			}
			for k, v := range keys {
				result[k] = v
			}

		case *hclsyntax.RelativeTraversalExpr, *hclsyntax.ScopeTraversalExpr:
			// dependency.X.outputs are handled separately via ExtractDepRefs
			if depNameFromTraversal(a) != "" {
				continue
			}
			// Other traversals (e.g., include.root.locals.project) —
			// evaluate to discover keys if we have a context.
			if ctx == nil {
				continue
			}
			val, diags := a.Value(ctx)
			if diags.HasErrors() || !val.IsKnown() {
				continue
			}
			if val.Type().IsObjectType() || val.Type().IsMapType() {
				for it := val.ElementIterator(); it.Next(); {
					k, v := it.Element()
					if k.Type() == cty.String {
						// Store a synthetic literal expression for the value
						result[k.AsString()] = hcl.StaticExpr(v, arg.Range())
					}
				}
			}

		default:
			// Unknown argument type in merge — skip
			continue
		}
	}

	return result, nil
}

// ExtractDepRefs returns the names of dependencies referenced as
// dependency.<name>.outputs in merge() calls within an inputs expression.
func ExtractDepRefs(expr hcl.Expression) []string {
	synExpr, ok := expr.(hclsyntax.Expression)
	if !ok {
		return nil
	}

	fn, ok := synExpr.(*hclsyntax.FunctionCallExpr)
	if !ok || fn.Name != "merge" {
		return nil
	}

	var refs []string
	seen := make(map[string]bool)
	for _, arg := range fn.Args {
		name := depNameFromTraversal(arg)
		if name != "" && !seen[name] {
			refs = append(refs, name)
			seen[name] = true
		}
	}
	return refs
}

// depNameFromTraversal checks if an expression is `dependency.<name>.outputs`
// and returns the dep name, or "" if it doesn't match.
func depNameFromTraversal(expr hclsyntax.Expression) string {
	trav, ok := expr.(*hclsyntax.ScopeTraversalExpr)
	if !ok {
		return ""
	}
	if len(trav.Traversal) < 3 {
		return ""
	}
	root, ok := trav.Traversal[0].(hcl.TraverseRoot)
	if !ok || root.Name != "dependency" {
		return ""
	}
	attr1, ok := trav.Traversal[1].(hcl.TraverseAttr)
	if !ok {
		return ""
	}
	attr2, ok := trav.Traversal[2].(hcl.TraverseAttr)
	if !ok || attr2.Name != "outputs" {
		return ""
	}
	return attr1.Name
}
