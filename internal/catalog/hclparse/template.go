package hclparse

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"dangernoodle.io/terranoodle/internal/hclutils"
)

// TemplateDef holds the complete parsed template file.
type TemplateDef struct {
	CatalogSource string               // terraform.source value
	Locals        map[string]cty.Value // evaluated locals
	Stacks        []UnitDef
	Path          string   // absolute path to template file
	IgnoreDeps    []string // dependency labels to ignore during dep validation (merged with catalog config)
	NameMustMatch string   // values key that must equal the template name (overrides catalog config)
}

// UnitDef holds a single template block — one tenant/unit entry within a template
// definition file. Named UnitDef to distinguish it from hclutils' StackConfig
// type, which has different semantics.
type UnitDef struct {
	Name      string
	Values    map[string]cty.Value      // fully evaluated values tree
	RawValues map[string]hcl.Expression // raw expressions for template.xxx resolution
}

var templateDefFileSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "config"},
		{Type: "terraform"},
		{Type: "locals"},
		{Type: "template", LabelNames: []string{"name"}},
	},
}

var templateConfigBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "ignore_deps"},
		{Name: "name_must_match"},
	},
}

var terraformBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "source"},
	},
}

var templateBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "values"},
	},
}

// ParseTemplateFile parses a catalog template definition HCL file.
// Returns the parsed definition, any warnings (e.g. unresolved locals), and any error.
func ParseTemplateFile(path string) (*TemplateDef, []string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving path %s: %w", path, err)
	}

	src, err := os.ReadFile(absPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", absPath, err)
	}

	file, diags := hclsyntax.ParseConfig(src, absPath, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, nil, fmt.Errorf("parsing %s: %s", absPath, diags.Error())
	}

	content, _, diags := file.Body.PartialContent(templateDefFileSchema)
	if diags.HasErrors() {
		return nil, nil, fmt.Errorf("decoding %s: %s", absPath, diags.Error())
	}

	// Build eval context with standard functions.
	ctx := hclutils.EvalContext(absPath)
	if ctx.Variables == nil {
		ctx.Variables = map[string]cty.Value{}
	}

	var warnings []string

	// Evaluate locals blocks and add local.* to context.
	var locals map[string]cty.Value
	for _, block := range content.Blocks {
		if block.Type == "locals" {
			locals = hclutils.EvalLocals(block.Body, ctx)
			// Warn about locals that could not be fully evaluated (cty.DynamicVal).
			for name, val := range locals {
				if val == cty.DynamicVal {
					warnings = append(warnings, fmt.Sprintf("local %q could not be fully evaluated in %s", name, absPath))
				}
			}
			if len(locals) > 0 {
				ctx.Variables["local"] = cty.ObjectVal(locals)
			}
		}
	}

	def := &TemplateDef{
		Path:   absPath,
		Locals: locals,
	}

	// Parse config block for ignore_deps.
	for _, block := range content.Blocks {
		if block.Type == "config" {
			blockContent, _, diags := block.Body.PartialContent(templateConfigBlockSchema)
			if diags.HasErrors() {
				return nil, nil, fmt.Errorf("decoding config block in %s: %s", absPath, diags.Error())
			}
			if attr, ok := blockContent.Attributes["ignore_deps"]; ok {
				val, diags := attr.Expr.Value(ctx)
				if diags.HasErrors() {
					val, diags = attr.Expr.Value(nil)
					if diags.HasErrors() {
						return nil, nil, fmt.Errorf("evaluating ignore_deps in %s: %s", absPath, diags.Error())
					}
				}
				if val.IsKnown() && !val.IsNull() {
					vType := val.Type()
					if !vType.IsListType() && !vType.IsTupleType() {
						return nil, nil, fmt.Errorf("ignore_deps must be a list of strings in %s", absPath)
					}
					for it := val.ElementIterator(); it.Next(); {
						_, lv := it.Element()
						if lv.Type() != cty.String {
							return nil, nil, fmt.Errorf("ignore_deps elements must be strings in %s", absPath)
						}
						def.IgnoreDeps = append(def.IgnoreDeps, lv.AsString())
					}
				}
			}
			if attr, ok := blockContent.Attributes["name_must_match"]; ok {
				val, diags := attr.Expr.Value(ctx)
				if diags.HasErrors() {
					val, diags = attr.Expr.Value(nil)
					if diags.HasErrors() {
						return nil, nil, fmt.Errorf("evaluating name_must_match in %s: %s", absPath, diags.Error())
					}
				}
				if val.IsKnown() && !val.IsNull() {
					if val.Type() != cty.String {
						return nil, nil, fmt.Errorf("name_must_match must be a string in %s", absPath)
					}
					def.NameMustMatch = val.AsString()
				}
			}
			break
		}
	}

	// Extract terraform.source.
	for _, block := range content.Blocks {
		if block.Type == "terraform" {
			source, err := extractTerraformSource(block.Body, ctx, absPath)
			if err != nil {
				return nil, nil, err
			}
			def.CatalogSource = source
		}
	}

	// Parse template blocks.
	seen := make(map[string]bool)
	for _, block := range content.Blocks {
		if block.Type != "template" {
			continue
		}

		templateName := block.Labels[0]

		// Validation 2: empty/blank template names.
		if strings.TrimSpace(templateName) == "" {
			return nil, nil, fmt.Errorf("template name must not be empty in %s", absPath)
		}

		// Validation 1: duplicate template names.
		if seen[templateName] {
			return nil, nil, fmt.Errorf("duplicate template name %q in %s", templateName, absPath)
		}
		seen[templateName] = true

		templateContent, _, diags := block.Body.PartialContent(templateBlockSchema)
		if diags.HasErrors() {
			return nil, nil, fmt.Errorf("decoding template %q in %s: %s", templateName, absPath, diags.Error())
		}

		cfg := UnitDef{Name: templateName}

		// Parse values attribute — store both raw expressions and evaluated values.
		if attr, ok := templateContent.Attributes["values"]; ok {
			rawValues, evaluated, err := parseValues(attr.Expr, ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("template %q values: %w", templateName, err)
			}
			cfg.RawValues = rawValues
			cfg.Values = evaluated
		}

		def.Stacks = append(def.Stacks, cfg)
	}

	return def, warnings, nil
}

// extractTerraformSource extracts the source attribute from a terraform block.
func extractTerraformSource(body hcl.Body, ctx *hcl.EvalContext, filePath string) (string, error) {
	blockContent, _, diags := body.PartialContent(terraformBlockSchema)
	if diags.HasErrors() {
		return "", fmt.Errorf("decoding terraform block in %s: %s", filePath, diags.Error())
	}

	attr, ok := blockContent.Attributes["source"]
	if !ok {
		return "", nil
	}

	val, diags := attr.Expr.Value(ctx)
	if diags.HasErrors() {
		return "", fmt.Errorf("evaluating terraform.source in %s: %s", filePath, diags.Error())
	}

	if val.Type() != cty.String {
		return "", fmt.Errorf("terraform.source must be a string in %s", filePath)
	}

	return val.AsString(), nil
}

// parseValues parses the values attribute, returning both raw expressions
// (for later template.xxx resolution) and eagerly-evaluated cty values for
// expressions that don't reference other templates.
func parseValues(expr hcl.Expression, ctx *hcl.EvalContext) (map[string]hcl.Expression, map[string]cty.Value, error) {
	synExpr, ok := expr.(hclsyntax.Expression)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected values expression type %T", expr)
	}

	obj, ok := synExpr.(*hclsyntax.ObjectConsExpr)
	if !ok {
		return nil, nil, fmt.Errorf("values must be an object literal, got %T", synExpr)
	}

	rawValues := make(map[string]hcl.Expression)
	evaluated := make(map[string]cty.Value)

	for _, item := range obj.Items {
		keyVal, diags := item.KeyExpr.Value(nil)
		if diags.HasErrors() {
			continue
		}
		if keyVal.Type() != cty.String {
			continue
		}
		key := keyVal.AsString()

		rawValues[key] = item.ValueExpr

		// Attempt eager evaluation; skip if it references template.xxx.
		if !containsTemplateRef(item.ValueExpr) {
			val, diags := item.ValueExpr.Value(ctx)
			if !diags.HasErrors() && val.IsKnown() {
				evaluated[key] = val
			}
		}
	}

	return rawValues, evaluated, nil
}

// containsTemplateRef returns true if the expression or any sub-expression
// references the "template" traversal root (i.e., template.xxx references).
func containsTemplateRef(expr hclsyntax.Expression) bool {
	for _, traversal := range expr.Variables() {
		if len(traversal) > 0 {
			if root, ok := traversal[0].(hcl.TraverseRoot); ok && root.Name == "template" {
				return true
			}
		}
	}
	return false
}
