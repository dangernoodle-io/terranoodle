package tfmod

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// HasOptionalWithoutDefault returns true if the type expression contains any
// optional() call without a default value argument (only 1 arg instead of 2).
func HasOptionalWithoutDefault(expr hcl.Expression) bool {
	if expr == nil {
		return false
	}

	synExpr, ok := expr.(hclsyntax.Expression)
	if !ok {
		return false
	}

	return walkOptional(synExpr)
}

func walkOptional(expr hclsyntax.Expression) bool {
	switch e := expr.(type) {
	case *hclsyntax.FunctionCallExpr:
		if e.Name == "optional" {
			if len(e.Args) == 1 {
				return true
			}
			// optional() with 2+ args has a default; still recurse into first arg
			// in case it's object({nested = optional(string)})
			if len(e.Args) >= 2 {
				return walkOptional(e.Args[0])
			}
			return false
		}
		// object() or other function calls — recurse into all args
		for _, arg := range e.Args {
			if walkOptional(arg) {
				return true
			}
		}
	case *hclsyntax.ObjectConsExpr:
		for _, item := range e.Items {
			if walkOptional(item.ValueExpr) {
				return true
			}
		}
	}
	return false
}
