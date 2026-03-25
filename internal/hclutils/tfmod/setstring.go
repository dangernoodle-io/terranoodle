package tfmod

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// HasSetString returns true if the type expression contains set(string) anywhere.
func HasSetString(expr hcl.Expression) bool {
	if expr == nil {
		return false
	}

	synExpr, ok := expr.(hclsyntax.Expression)
	if !ok {
		return false
	}

	return walkSetString(synExpr)
}

func walkSetString(expr hclsyntax.Expression) bool {
	switch e := expr.(type) {
	case *hclsyntax.FunctionCallExpr:
		if e.Name == "set" && len(e.Args) == 1 {
			// Check if the arg is the bare keyword "string"
			if scope, ok := e.Args[0].(*hclsyntax.ScopeTraversalExpr); ok {
				if len(scope.Traversal) == 1 && scope.Traversal.RootName() == "string" {
					return true
				}
			}
		}
		// Recurse into all args for nested types
		for _, arg := range e.Args {
			if walkSetString(arg) {
				return true
			}
		}
	case *hclsyntax.ObjectConsExpr:
		for _, item := range e.Items {
			if walkSetString(item.ValueExpr) {
				return true
			}
		}
	}
	return false
}
