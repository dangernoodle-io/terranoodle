package tfmod

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// Output represents a declared terraform output.
type Output struct {
	Name           string
	HasDescription bool
	IsSensitive    bool
	VarRefs        []string // variable names referenced via var.X
}

// ParseOutputs reads all .tf files in a module directory and returns outputs.
func ParseOutputs(moduleDir string) ([]Output, error) {
	entries, err := os.ReadDir(moduleDir)
	if err != nil {
		return nil, fmt.Errorf("reading module dir %s: %w", moduleDir, err)
	}

	var outputs []Output

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

		outs, err := extractOutputs(file.Body)
		if err != nil {
			return nil, fmt.Errorf("extracting outputs from %s: %w", path, err)
		}
		outputs = append(outputs, outs...)
	}

	return outputs, nil
}

var outputBlockSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "output", LabelNames: []string{"name"}},
	},
}

var outputBodySchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "value"},
		{Name: "description"},
		{Name: "sensitive"},
	},
}

func extractOutputs(body hcl.Body) ([]Output, error) {
	content, _, diags := body.PartialContent(outputBlockSchema)
	if diags.HasErrors() {
		return nil, fmt.Errorf("decoding: %s", diags.Error())
	}

	var outputs []Output
	for _, block := range content.Blocks {
		if block.Type != "output" {
			continue
		}
		bodyContent, _, diags := block.Body.PartialContent(outputBodySchema)
		if diags.HasErrors() {
			return nil, fmt.Errorf("decoding output %s: %s", block.Labels[0], diags.Error())
		}
		o := Output{Name: block.Labels[0]}
		if _, ok := bodyContent.Attributes["description"]; ok {
			o.HasDescription = true
		}
		if attr, ok := bodyContent.Attributes["sensitive"]; ok {
			val, _ := attr.Expr.Value(nil)
			if val == cty.True {
				o.IsSensitive = true
			}
		}
		if attr, ok := bodyContent.Attributes["value"]; ok {
			traversals := attr.Expr.Variables()
			for _, traversal := range traversals {
				if len(traversal) >= 2 {
					if root, ok := traversal[0].(hcl.TraverseRoot); ok && root.Name == "var" {
						if attrStep, ok := traversal[1].(hcl.TraverseAttr); ok {
							o.VarRefs = append(o.VarRefs, attrStep.Name)
						}
					}
				}
			}
		}
		outputs = append(outputs, o)
	}
	return outputs, nil
}
