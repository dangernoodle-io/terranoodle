package hclutils

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// TerragruntConfig holds the parsed components of a terragrunt.hcl file.
type TerragruntConfig struct {
	Source             string
	Inputs             map[string]hcl.Expression
	DepRefs            []string           // dep names referenced via dependency.<name>.outputs in merge()
	Dependencies       []DependencyConfig // all parsed dependency blocks
	EvalCtx            *hcl.EvalContext   // context used for evaluating input expressions
	Path               string             // absolute path to the parsed file
	IncludeInputKeys   map[string]bool    // input keys automatically merged from parent includes
	TfVarFiles         []string           // resolved paths to terraform var files from extra_arguments
	Includes           []IncludeConfig    // all parsed include blocks
	ProviderBlockNames []string           // names of provider blocks found in the config
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
		{Type: "provider", LabelNames: []string{"name"}},
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

var extraArgumentsBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "optional_var_files"},
		{Name: "required_var_files"},
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

	configDir := filepath.Dir(path)

	// Collect tfvars files from config and all includes
	tfVarFileMap := make(map[string]bool)

	// Add from this config's extra_arguments
	for _, f := range extractTfVarFiles(content.Blocks, ctx, configDir) {
		tfVarFileMap[f] = true
	}

	// Add from all includes' extra_arguments
	for _, inc := range includes {
		if inc.Path == "" {
			continue
		}
		includePath := inc.Path
		if !filepath.IsAbs(includePath) {
			includePath = filepath.Join(configDir, includePath)
		}
		extraArgs, err := ResolveIncludeExtraArgs(includePath, configDir)
		if err == nil {
			for _, f := range extraArgs {
				tfVarFileMap[f] = true
			}
		}
	}

	// Convert to sorted slice
	tfVarFiles := make([]string, 0, len(tfVarFileMap))
	for f := range tfVarFileMap {
		tfVarFiles = append(tfVarFiles, f)
	}
	sort.Strings(tfVarFiles)

	cfg := &TerragruntConfig{
		Path:             path,
		EvalCtx:          ctx,
		IncludeInputKeys: MergedIncludeInputKeys(includes, configDir),
		TfVarFiles:       tfVarFiles,
		Includes:         includes,
	}

	// Phase 4: Parse dependency blocks
	deps, err := ParseDependencies(content.Blocks, ctx, filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	cfg.Dependencies = deps

	// Phase 5: Collect provider block names
	for _, block := range content.Blocks {
		if block.Type == "provider" {
			if len(block.Labels) > 0 {
				cfg.ProviderBlockNames = append(cfg.ProviderBlockNames, block.Labels[0])
			}
		}
	}

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

// extractTfVarFiles finds terraform.tfvars and *.auto.tfvars files in configDir,
// and evaluates optional_var_files/required_var_files from extra_arguments blocks.
// Returns the resolved absolute paths.
func extractTfVarFiles(blocks []*hcl.Block, ctx *hcl.EvalContext, configDir string) []string {
	filePaths := make(map[string]bool)

	// Auto-detect terraform.tfvars and *.auto.tfvars in configDir
	if entries, err := os.ReadDir(configDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if name == "terraform.tfvars" || strings.HasSuffix(name, ".auto.tfvars") {
				filePaths[filepath.Join(configDir, name)] = true
			}
		}
	}

	// Parse extra_arguments blocks from terraform block
	for _, block := range blocks {
		if block.Type != "terraform" {
			continue
		}

		// Find extra_arguments sub-blocks
		content, _, diags := block.Body.PartialContent(&hcl.BodySchema{
			Blocks: []hcl.BlockHeaderSchema{
				{Type: "extra_arguments", LabelNames: []string{"name"}},
			},
		})
		if diags.HasErrors() {
			continue
		}

		for _, extraBlock := range content.Blocks {
			if extraBlock.Type != "extra_arguments" {
				continue
			}

			extraContent, _, diags := extraBlock.Body.PartialContent(extraArgumentsBlockSchema)
			if diags.HasErrors() {
				continue
			}

			// Evaluate optional_var_files
			if attr, ok := extraContent.Attributes["optional_var_files"]; ok {
				val, diags := attr.Expr.Value(ctx)
				if !diags.HasErrors() && val.IsKnown() && (val.Type().IsListType() || val.Type().IsSetType()) {
					for _, item := range val.AsValueSlice() {
						if item.Type() == cty.String {
							path := item.AsString()
							if !filepath.IsAbs(path) {
								path = filepath.Join(configDir, path)
							}
							filePaths[filepath.Clean(path)] = true
						}
					}
				}
			}

			// Evaluate required_var_files
			if attr, ok := extraContent.Attributes["required_var_files"]; ok {
				val, diags := attr.Expr.Value(ctx)
				if !diags.HasErrors() && val.IsKnown() && (val.Type().IsListType() || val.Type().IsSetType()) {
					for _, item := range val.AsValueSlice() {
						if item.Type() == cty.String {
							path := item.AsString()
							if !filepath.IsAbs(path) {
								path = filepath.Join(configDir, path)
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
	return result
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
