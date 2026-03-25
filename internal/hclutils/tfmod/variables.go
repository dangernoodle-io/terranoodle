package tfmod

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// Variable represents a declared terraform variable.
type Variable struct {
	Name           string
	HasDefault     bool
	HasDescription bool
	HasValidation  bool
	IsSensitive    bool
	Type           hcl.Expression // raw type expression, nil if unspecified (Phase 5)
}

// ParseVariables reads all .tf files in a module directory and extracts
// variable declarations with their name and whether they have a default value.
func ParseVariables(moduleDir string) ([]Variable, error) {
	entries, err := os.ReadDir(moduleDir)
	if err != nil {
		return nil, fmt.Errorf("reading module dir %s: %w", moduleDir, err)
	}

	var variables []Variable

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

		vars, err := extractVariables(file.Body)
		if err != nil {
			return nil, fmt.Errorf("extracting variables from %s: %w", path, err)
		}
		variables = append(variables, vars...)
	}

	return variables, nil
}

var variableBlockSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "variable", LabelNames: []string{"name"}},
	},
}

var variableBodySchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "default"},
		{Name: "type"},
		{Name: "description"},
		{Name: "sensitive"},
	},
}

var variableValidationSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "validation"},
	},
}

func extractVariables(body hcl.Body) ([]Variable, error) {
	content, _, diags := body.PartialContent(variableBlockSchema)
	if diags.HasErrors() {
		return nil, fmt.Errorf("decoding: %s", diags.Error())
	}

	var vars []Variable
	for _, block := range content.Blocks {
		if block.Type != "variable" {
			continue
		}

		varContent, remain, diags := block.Body.PartialContent(variableBodySchema)
		if diags.HasErrors() {
			return nil, fmt.Errorf("decoding variable %s: %s", block.Labels[0], diags.Error())
		}

		v := Variable{
			Name: block.Labels[0],
		}

		if _, ok := varContent.Attributes["default"]; ok {
			v.HasDefault = true
		}
		if _, ok := varContent.Attributes["description"]; ok {
			v.HasDescription = true
		}
		if attr, ok := varContent.Attributes["type"]; ok {
			v.Type = attr.Expr
		}
		if attr, ok := varContent.Attributes["sensitive"]; ok {
			val, _ := attr.Expr.Value(nil)
			if val == cty.True {
				v.IsSensitive = true
			}
		}

		// Check for validation blocks in remaining body
		valContent, _, _ := remain.PartialContent(variableValidationSchema)
		if len(valContent.Blocks) > 0 {
			v.HasValidation = true
		}

		vars = append(vars, v)
	}

	return vars, nil
}
