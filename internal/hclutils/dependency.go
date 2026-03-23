package hclutils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// DependencyConfig represents a parsed dependency block.
type DependencyConfig struct {
	Name       string
	ConfigPath string // resolved absolute path to the dependency directory
}

var dependencyBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "config_path", Required: true},
	},
}

// ParseDependencies extracts dependency blocks and resolves their config paths
// relative to the directory of the config file being parsed.
func ParseDependencies(blocks []*hcl.Block, ctx *hcl.EvalContext, configDir string) ([]DependencyConfig, error) {
	var deps []DependencyConfig

	for _, block := range blocks {
		if block.Type != "dependency" {
			continue
		}

		content, _, diags := block.Body.PartialContent(dependencyBlockSchema)
		if diags.HasErrors() {
			continue
		}

		dep := DependencyConfig{Name: block.Labels[0]}

		if attr, ok := content.Attributes["config_path"]; ok {
			val, diags := attr.Expr.Value(ctx)
			if diags.HasErrors() {
				return nil, fmt.Errorf("evaluating dependency %q config_path: %s", dep.Name, diags.Error())
			}
			if !val.IsKnown() || val.Type() != cty.String {
				continue
			}
			p := val.AsString()
			if !filepath.IsAbs(p) {
				p = filepath.Join(configDir, p)
			}
			dep.ConfigPath = filepath.Clean(p)
		}

		if dep.ConfigPath == "" {
			continue
		}

		deps = append(deps, dep)
	}

	return deps, nil
}

// ParseDependencyLabels reads a terragrunt.hcl file and returns the labels
// from all dependency blocks. This is a lightweight parse that only extracts
// block labels without evaluating expressions or resolving paths.
func ParseDependencyLabels(path string) ([]string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	file, diags := hclsyntax.ParseConfig(src, path, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, diags
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("unexpected body type %T", file.Body)
	}

	var labels []string
	for _, block := range body.Blocks {
		if block.Type == "dependency" && len(block.Labels) > 0 {
			labels = append(labels, block.Labels[0])
		}
	}

	return labels, nil
}
