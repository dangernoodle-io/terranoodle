package hclutils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// IncludeConfig represents a parsed include block.
type IncludeConfig struct {
	Name   string
	Path   string
	Expose bool
}

var includeBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "path", Required: true},
		{Name: "expose"},
	},
}

// ParseIncludes extracts include blocks and resolves their paths.
func ParseIncludes(blocks []*hcl.Block, ctx *hcl.EvalContext) ([]IncludeConfig, error) {
	var includes []IncludeConfig

	for _, block := range blocks {
		if block.Type != "include" {
			continue
		}

		content, _, diags := block.Body.PartialContent(includeBlockSchema)
		if diags.HasErrors() {
			continue
		}

		inc := IncludeConfig{Name: block.Labels[0]}

		if attr, ok := content.Attributes["path"]; ok {
			val, diags := attr.Expr.Value(ctx)
			if diags.HasErrors() {
				return nil, fmt.Errorf("evaluating include %q path: %s", inc.Name, diags.Error())
			}
			if !val.IsKnown() || val.AsString() == "" {
				continue
			}
			inc.Path = val.AsString()
		}

		if attr, ok := content.Attributes["expose"]; ok {
			val, diags := attr.Expr.Value(ctx)
			if !diags.HasErrors() && val.Type() == cty.Bool {
				inc.Expose = val.True()
			}
		}

		includes = append(includes, inc)
	}

	return includes, nil
}

// ResolveIncludeLocals parses an included file and evaluates its locals block.
// Returns the locals as a cty.Value object, or cty.EmptyObjectVal if none.
func ResolveIncludeLocals(includePath string) (cty.Value, error) {
	absPath, err := filepath.Abs(includePath)
	if err != nil {
		return cty.EmptyObjectVal, fmt.Errorf("resolving include path: %w", err)
	}

	src, err := os.ReadFile(absPath)
	if err != nil {
		return cty.EmptyObjectVal, fmt.Errorf("reading include %s: %w", absPath, err)
	}

	file, diags := hclsyntax.ParseConfig(src, absPath, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return cty.EmptyObjectVal, fmt.Errorf("parsing include %s: %s", absPath, diags.Error())
	}

	// Build an eval context for the included file (scoped to its own directory)
	ctx := EvalContext(absPath)
	if ctx.Variables == nil {
		ctx.Variables = map[string]cty.Value{}
	}

	content, _, diags := file.Body.PartialContent(configFileSchema)
	if diags.HasErrors() {
		return cty.EmptyObjectVal, nil
	}

	// Evaluate locals in the included file
	for _, block := range content.Blocks {
		if block.Type == "locals" {
			locals := EvalLocals(block.Body, ctx)
			if len(locals) > 0 {
				return cty.ObjectVal(locals), nil
			}
		}
	}

	return cty.EmptyObjectVal, nil
}

// BuildIncludeVariable constructs the `include` variable for the eval context.
// Each include with expose=true gets its locals available as include.<name>.locals.*.
func BuildIncludeVariable(includes []IncludeConfig) cty.Value {
	incMap := make(map[string]cty.Value)

	for _, inc := range includes {
		if !inc.Expose || inc.Path == "" {
			continue
		}

		locals, err := ResolveIncludeLocals(inc.Path)
		if err != nil {
			continue
		}

		incMap[inc.Name] = cty.ObjectVal(map[string]cty.Value{
			"locals": locals,
		})
	}

	if len(incMap) == 0 {
		return cty.EmptyObjectVal
	}
	return cty.ObjectVal(incMap)
}
