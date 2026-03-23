package hclutils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// TerragruntConfig holds the parsed components of a terragrunt.hcl file.
type TerragruntConfig struct {
	Source       string
	Inputs       map[string]hcl.Expression
	DepRefs      []string           // dep names referenced via dependency.<name>.outputs in merge()
	Dependencies []DependencyConfig // all parsed dependency blocks
	EvalCtx      *hcl.EvalContext   // context used for evaluating input expressions
	Path         string             // absolute path to the parsed file
}

// configFileSchema defines the top-level blocks we expect in a terragrunt.hcl.
var configFileSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "terraform"},
		{Type: "locals"},
		{Type: "include", LabelNames: []string{"name"}},
		{Type: "dependency", LabelNames: []string{"name"}},
		{Type: "dependencies"},
		{Type: "generate", LabelNames: []string{"name"}},
	},
	Attributes: []hcl.AttributeSchema{
		{Name: "inputs"},
	},
}

var terraformBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "source"},
	},
}

// ParseFile parses a terragrunt.hcl file and extracts the terraform source and inputs.
func ParseFile(path string) (*TerragruntConfig, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	file, diags := hclsyntax.ParseConfig(src, path, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parsing %s: %s", path, diags.Error())
	}

	return parseBody(file.Body, path)
}

func parseBody(body hcl.Body, path string) (*TerragruntConfig, error) {
	content, _, diags := body.PartialContent(configFileSchema)
	if diags.HasErrors() {
		return nil, fmt.Errorf("decoding %s: %s", path, diags.Error())
	}

	// Phase 1: Build base eval context with functions
	ctx := EvalContext(path)
	if ctx.Variables == nil {
		ctx.Variables = map[string]cty.Value{}
	}

	// Phase 2: Evaluate locals → add local.* to context
	for _, block := range content.Blocks {
		if block.Type == "locals" {
			locals := EvalLocals(block.Body, ctx)
			if len(locals) > 0 {
				ctx.Variables["local"] = cty.ObjectVal(locals)
			}
		}
	}

	// Phase 3: Resolve includes → add include.* to context
	includes, err := ParseIncludes(content.Blocks, ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(includes) > 0 {
		incVar := BuildIncludeVariable(includes)
		if incVar.IsKnown() && incVar.Type().IsObjectType() && incVar.LengthInt() > 0 {
			ctx.Variables["include"] = incVar
		}
	}

	cfg := &TerragruntConfig{Path: path, EvalCtx: ctx}

	// Phase 4: Parse dependency blocks
	deps, err := ParseDependencies(content.Blocks, ctx, filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	cfg.Dependencies = deps

	// Extract terraform.source (with full context)
	for _, block := range content.Blocks {
		if block.Type == "terraform" {
			source, err := extractSource(block.Body, ctx)
			if err != nil {
				return nil, err
			}
			cfg.Source = source
		}
	}

	// Extract inputs
	if attr, ok := content.Attributes["inputs"]; ok {
		inputs, err := ExtractInputKeys(attr.Expr, ctx)
		if err != nil {
			return nil, fmt.Errorf("extracting inputs from %s: %w", path, err)
		}
		cfg.Inputs = inputs
		cfg.DepRefs = ExtractDepRefs(attr.Expr)
	}

	return cfg, nil
}

func extractSource(body hcl.Body, ctx *hcl.EvalContext) (string, error) {
	content, _, diags := body.PartialContent(terraformBlockSchema)
	if diags.HasErrors() {
		return "", fmt.Errorf("decoding terraform block: %s", diags.Error())
	}

	attr, ok := content.Attributes["source"]
	if !ok {
		return "", nil
	}

	val, diags := attr.Expr.Value(ctx)
	if diags.HasErrors() {
		return "", fmt.Errorf("evaluating terraform.source: %s", diags.Error())
	}

	return val.AsString(), nil
}
