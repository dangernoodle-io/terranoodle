package hclutils

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

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

// ResolveIncludeInputs parses an included file and evaluates its inputs attribute.
// Returns the inputs as a cty.Value object, or cty.EmptyObjectVal if none.
// Inputs may reference locals from the included file, so locals are evaluated first.
func ResolveIncludeInputs(includePath string) (cty.Value, error) {
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

	// Evaluate locals in the included file first (inputs may reference them)
	for _, block := range content.Blocks {
		if block.Type == "locals" {
			locals := EvalLocals(block.Body, ctx)
			if len(locals) > 0 {
				ctx.Variables["local"] = cty.ObjectVal(locals)
				break
			}
		}
	}

	// Evaluate inputs attribute
	if attr, ok := content.Attributes["inputs"]; ok {
		val, diags := attr.Expr.Value(ctx)
		if !diags.HasErrors() && val.IsKnown() && (val.Type().IsObjectType() || val.Type().IsMapType()) {
			return val, nil
		}
	}

	return cty.EmptyObjectVal, nil
}

// BuildIncludeVariable constructs the `include` variable for the eval context.
// Each include with expose=true gets its locals and inputs available as include.<name>.locals.* and include.<name>.inputs.*.
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

		inputs, err := ResolveIncludeInputs(inc.Path)
		if err != nil {
			inputs = cty.EmptyObjectVal
		}

		incMap[inc.Name] = cty.ObjectVal(map[string]cty.Value{
			"locals": locals,
			"inputs": inputs,
		})
	}

	if len(incMap) == 0 {
		return cty.EmptyObjectVal
	}
	return cty.ObjectVal(incMap)
}

// MergedIncludeInputKeys returns a map of all input keys from all includes.
// This represents inputs that are automatically merged from parent configs.
// configDir is the directory containing the terragrunt.hcl file being parsed.
func MergedIncludeInputKeys(includes []IncludeConfig, configDir string) map[string]bool {
	keys := make(map[string]bool)
	for _, inc := range includes {
		if inc.Path == "" {
			continue
		}
		// Resolve include path relative to the config directory
		includePath := inc.Path
		if !filepath.IsAbs(includePath) {
			includePath = filepath.Join(configDir, includePath)
		}
		inputs, err := ResolveIncludeInputs(includePath)
		if err != nil || inputs.Equals(cty.EmptyObjectVal).True() {
			continue
		}
		for k := range inputs.AsValueMap() {
			keys[k] = true
		}
	}
	return keys
}

// ResolveIncludeExtraArgs parses an included file and evaluates its terraform.extra_arguments blocks.
// It returns resolved paths from optional_var_files and required_var_files.
// childDir is the directory of the child (leaf) config — it's used for get_terragrunt_dir() resolution.
// This matches Terragrunt's behavior where get_terragrunt_dir() in an include returns the child config's dir.
func ResolveIncludeExtraArgs(includePath string, childDir string) ([]string, error) {
	absPath, err := filepath.Abs(includePath)
	if err != nil {
		return nil, fmt.Errorf("resolving include path: %w", err)
	}

	src, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("reading include %s: %w", absPath, err)
	}

	file, diags := hclsyntax.ParseConfig(src, absPath, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parsing include %s: %s", absPath, diags.Error())
	}

	// Build eval context with childDir for get_terragrunt_dir() — critical for correct path resolution
	ctx := EvalContext(childDir)
	if ctx.Variables == nil {
		ctx.Variables = map[string]cty.Value{}
	}

	content, _, diags := file.Body.PartialContent(configFileSchema)
	if diags.HasErrors() {
		return nil, nil
	}

	// Evaluate locals in the included file first
	for _, block := range content.Blocks {
		if block.Type == "locals" {
			locals := EvalLocals(block.Body, ctx)
			if len(locals) > 0 {
				ctx.Variables["local"] = cty.ObjectVal(locals)
				break
			}
		}
	}

	// Find terraform blocks and extract extra_arguments
	filePaths := make(map[string]bool)
	for _, block := range content.Blocks {
		if block.Type != "terraform" {
			continue
		}

		// Find extra_arguments sub-blocks
		extraSchema := &hcl.BodySchema{
			Blocks: []hcl.BlockHeaderSchema{
				{Type: "extra_arguments", LabelNames: []string{"name"}},
			},
		}
		extraContent, _, diags := block.Body.PartialContent(extraSchema)
		if diags.HasErrors() {
			continue
		}

		for _, extraBlock := range extraContent.Blocks {
			if extraBlock.Type != "extra_arguments" {
				continue
			}

			extraArgSchema := &hcl.BodySchema{
				Attributes: []hcl.AttributeSchema{
					{Name: "optional_var_files"},
					{Name: "required_var_files"},
				},
			}
			argContent, _, diags := extraBlock.Body.PartialContent(extraArgSchema)
			if diags.HasErrors() {
				continue
			}

			// Evaluate optional_var_files
			if attr, ok := argContent.Attributes["optional_var_files"]; ok {
				val, diags := attr.Expr.Value(ctx)
				if !diags.HasErrors() && val.IsKnown() && (val.Type().IsListType() || val.Type().IsSetType()) {
					for _, item := range val.AsValueSlice() {
						if item.Type() == cty.String {
							path := item.AsString()
							// Resolve relative to include file's directory
							if !filepath.IsAbs(path) {
								path = filepath.Join(filepath.Dir(absPath), path)
							}
							filePaths[filepath.Clean(path)] = true
						}
					}
				}
			}

			// Evaluate required_var_files
			if attr, ok := argContent.Attributes["required_var_files"]; ok {
				val, diags := attr.Expr.Value(ctx)
				if !diags.HasErrors() && val.IsKnown() && (val.Type().IsListType() || val.Type().IsSetType()) {
					for _, item := range val.AsValueSlice() {
						if item.Type() == cty.String {
							path := item.AsString()
							// Resolve relative to include file's directory
							if !filepath.IsAbs(path) {
								path = filepath.Join(filepath.Dir(absPath), path)
							}
							filePaths[filepath.Clean(path)] = true
						}
					}
				}
			}
		}
	}

	// Convert map to sorted slice
	result := make([]string, 0, len(filePaths))
	for path := range filePaths {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, nil
}
