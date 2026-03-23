package tfmod

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// ParseOutputs reads all .tf files in a module directory and returns output names.
func ParseOutputs(moduleDir string) ([]string, error) {
	entries, err := os.ReadDir(moduleDir)
	if err != nil {
		return nil, fmt.Errorf("reading module dir %s: %w", moduleDir, err)
	}

	var outputs []string

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

		names, err := extractOutputNames(file.Body)
		if err != nil {
			return nil, fmt.Errorf("extracting outputs from %s: %w", path, err)
		}
		outputs = append(outputs, names...)
	}

	return outputs, nil
}

var outputBlockSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "output", LabelNames: []string{"name"}},
	},
}

func extractOutputNames(body hcl.Body) ([]string, error) {
	content, _, diags := body.PartialContent(outputBlockSchema)
	if diags.HasErrors() {
		return nil, fmt.Errorf("decoding: %s", diags.Error())
	}

	var names []string
	for _, block := range content.Blocks {
		if block.Type == "output" {
			names = append(names, block.Labels[0])
		}
	}
	return names, nil
}
