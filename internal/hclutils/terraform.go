package hclutils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// ModuleCall represents a parsed Terraform module block.
type ModuleCall struct {
	Name    string                    // block label
	Source  string                    // evaluated source attribute
	Inputs  map[string]hcl.Expression // all non-meta attributes
	EvalCtx *hcl.EvalContext
}

// moduleMetaSchema declares the meta-arguments that are not module inputs.
var moduleMetaSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "source"},
		{Name: "version"},
		{Name: "providers"},
		{Name: "depends_on"},
		{Name: "for_each"},
		{Name: "count"},
	},
}

// tfFileSchema extracts module, variable, and locals blocks from .tf files.
var tfFileSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "module", LabelNames: []string{"name"}},
		{Type: "variable", LabelNames: []string{"name"}},
		{Type: "locals"},
	},
}

// ParseModuleCalls reads all .tf files in a directory, evaluates variable and
// locals blocks to build an eval context, and extracts module blocks with their
// inputs.
func ParseModuleCalls(dir string) ([]ModuleCall, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving dir %s: %w", dir, err)
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, fmt.Errorf("reading dir %s: %w", absDir, err)
	}

	// Parse all .tf files and collect their bodies.
	var bodies []hcl.Body
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".tf" {
			continue
		}

		path := filepath.Join(absDir, entry.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}

		file, diags := hclsyntax.ParseConfig(src, path, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			return nil, fmt.Errorf("parsing %s: %s", path, diags.Error())
		}
		bodies = append(bodies, file.Body)
	}

	if len(bodies) == 0 {
		return nil, nil
	}

	// Build eval context from variable defaults and locals.
	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{},
	}

	// First pass: collect variable defaults for var.* context.
	vars := map[string]cty.Value{}
	for _, body := range bodies {
		content, _, diags := body.PartialContent(tfFileSchema)
		if diags.HasErrors() {
			continue
		}
		for _, block := range content.Blocks {
			if block.Type != "variable" {
				continue
			}
			name := block.Labels[0]
			defAttr, _ := block.Body.JustAttributes()
			if d, ok := defAttr["default"]; ok {
				val, diags := d.Expr.Value(nil)
				if !diags.HasErrors() {
					vars[name] = val
					continue
				}
			}
			// No default or unevaluable — use DynamicVal so references don't fail.
			vars[name] = cty.DynamicVal
		}
	}
	if len(vars) > 0 {
		ctx.Variables["var"] = cty.ObjectVal(vars)
	}

	// Second pass: evaluate locals.
	for _, body := range bodies {
		content, _, diags := body.PartialContent(tfFileSchema)
		if diags.HasErrors() {
			continue
		}
		for _, block := range content.Blocks {
			if block.Type == "locals" {
				EvalLocals(block.Body, ctx)
			}
		}
	}

	// Third pass: extract module blocks.
	var calls []ModuleCall
	for _, body := range bodies {
		content, _, diags := body.PartialContent(tfFileSchema)
		if diags.HasErrors() {
			continue
		}
		for _, block := range content.Blocks {
			if block.Type != "module" {
				continue
			}

			mc, err := parseModuleBlock(block, ctx)
			if err != nil {
				return nil, err
			}
			if mc != nil {
				calls = append(calls, *mc)
			}
		}
	}

	return calls, nil
}

func parseModuleBlock(block *hcl.Block, ctx *hcl.EvalContext) (*ModuleCall, error) {
	// Extract meta-arguments; the remaining body has the module inputs.
	metaContent, remain, diags := block.Body.PartialContent(moduleMetaSchema)
	if diags.HasErrors() {
		return nil, fmt.Errorf("decoding module %q: %s", block.Labels[0], diags.Error())
	}

	// Source is required for us to validate.
	sourceAttr, ok := metaContent.Attributes["source"]
	if !ok {
		return nil, nil // no source — skip
	}

	sourceVal, diags := sourceAttr.Expr.Value(ctx)
	if diags.HasErrors() || sourceVal.Type() != cty.String {
		return nil, nil // unresolvable source — skip
	}

	// Remaining attributes are module inputs.
	remainAttrs, diags := remain.JustAttributes()
	if diags.HasErrors() {
		return nil, fmt.Errorf("decoding inputs for module %q: %s", block.Labels[0], diags.Error())
	}

	inputs := make(map[string]hcl.Expression, len(remainAttrs))
	for name, attr := range remainAttrs {
		inputs[name] = attr.Expr
	}

	return &ModuleCall{
		Name:    block.Labels[0],
		Source:  sourceVal.AsString(),
		Inputs:  inputs,
		EvalCtx: ctx,
	}, nil
}
