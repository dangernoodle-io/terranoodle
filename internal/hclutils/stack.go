package hclutils

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// UnitConfig holds the parsed contents of a single unit block in a stack file.
type UnitConfig struct {
	Name    string                    // unit label (e.g., "bootstrap")
	Source  string                    // resolved source path
	Values  map[string]hcl.Expression // values block key→expression
	EvalCtx *hcl.EvalContext          // for evaluating value expressions
}

// StackConfig holds the parsed contents of a terragrunt.stack.hcl file.
type StackConfig struct {
	Units []UnitConfig
	Path  string // absolute path to the stack file
}

var stackFileSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "locals"},
		{Type: "unit", LabelNames: []string{"name"}},
	},
}

var unitBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "source"},
		{Name: "path"},
		{Name: "values"},
	},
}

// ParseStackFile parses a terragrunt.stack.hcl file and returns the units it defines.
func ParseStackFile(path string) (*StackConfig, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	file, diags := hclsyntax.ParseConfig(src, path, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parsing %s: %s", path, diags.Error())
	}

	content, _, diags := file.Body.PartialContent(stackFileSchema)
	if diags.HasErrors() {
		return nil, fmt.Errorf("decoding %s: %s", path, diags.Error())
	}

	// Build eval context with standard functions
	ctx := EvalContext(path)
	if ctx.Variables == nil {
		ctx.Variables = map[string]cty.Value{}
	}

	// Evaluate locals blocks and add local.* to context
	for _, block := range content.Blocks {
		if block.Type == "locals" {
			locals := EvalLocals(block.Body, ctx)
			if len(locals) > 0 {
				ctx.Variables["local"] = cty.ObjectVal(locals)
			}
		}
	}

	stack := &StackConfig{Path: path}

	// Parse unit blocks
	for _, block := range content.Blocks {
		if block.Type != "unit" {
			continue
		}

		unitName := block.Labels[0]

		unitContent, _, diags := block.Body.PartialContent(unitBlockSchema)
		if diags.HasErrors() {
			return nil, fmt.Errorf("decoding unit %q in %s: %s", unitName, path, diags.Error())
		}

		// Evaluate source attribute
		var source string
		if attr, ok := unitContent.Attributes["source"]; ok {
			val, diags := attr.Expr.Value(ctx)
			if !diags.HasErrors() && val.Type() == cty.String {
				source = val.AsString()
			}
		}

		// Extract values block
		var values map[string]hcl.Expression
		if attr, ok := unitContent.Attributes["values"]; ok {
			synExpr, ok := attr.Expr.(hclsyntax.Expression)
			if ok {
				if obj, ok := synExpr.(*hclsyntax.ObjectConsExpr); ok {
					v, err := ExtractObjectKeys(obj)
					if err != nil {
						return nil, fmt.Errorf("extracting values for unit %q: %w", unitName, err)
					}
					values = v
				}
			}
		}

		stack.Units = append(stack.Units, UnitConfig{
			Name:    unitName,
			Source:  source,
			Values:  values,
			EvalCtx: ctx,
		})
	}

	return stack, nil
}
