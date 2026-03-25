package tfmod

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// CollectVarRefs returns the set of var.<name> references across all .tf files in dir.
func CollectVarRefs(moduleDir string) (map[string]bool, error) {
	entries, err := os.ReadDir(moduleDir)
	if err != nil {
		return nil, fmt.Errorf("reading module dir %s: %w", moduleDir, err)
	}

	refs := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".tf" {
			continue
		}

		path := filepath.Join(moduleDir, entry.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}

		file, diags := hclsyntax.ParseConfig(src, path, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			return nil, fmt.Errorf("parsing %s: %s", path, diags.Error())
		}

		body, ok := file.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}

		// Walk the body attributes and collect var references from all expressions
		collectVarRefsFromBody(body, refs)
	}

	return refs, nil
}

func collectVarRefsFromBody(body *hclsyntax.Body, refs map[string]bool) {
	// Check all attributes in the body
	for _, attr := range body.Attributes {
		collectVarRefsFromExpr(attr.Expr, refs)
	}

	// Check all blocks (e.g., resource, module, etc.)
	for _, block := range body.Blocks {
		if block.Body != nil {
			collectVarRefsFromBody(block.Body, refs)
		}
	}
}

func collectVarRefsFromExpr(expr hclsyntax.Expression, refs map[string]bool) {
	// Get all variable references from this expression
	for _, traversal := range expr.Variables() {
		if len(traversal) >= 2 {
			root, ok := traversal[0].(hcl.TraverseRoot)
			if !ok || root.Name != "var" {
				continue
			}
			if step, ok := traversal[1].(hcl.TraverseAttr); ok {
				refs[step.Name] = true
			}
		}
	}
}
