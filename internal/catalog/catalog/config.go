package catalog

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"dangernoodle.io/terranoodle/internal/output"
)

// CatalogConfig holds configuration from the catalog's catalog config file.
type CatalogConfig struct {
	IgnoreDeps    []string // dependency labels to ignore during dep validation (not service deps)
	NameMustMatch string   // values key that must match template name (empty = no check)
}

var catalogConfigFileSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "config"},
	},
}

var catalogConfigBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "ignore_deps"},
		{Name: "name_must_match"},
	},
}

// ParseCatalogConfig reads an optional catalog config file at the catalog root.
// Returns an empty config (not an error) if the file does not exist.
func ParseCatalogConfig(catalogPath string) (*CatalogConfig, error) {
	configPath := filepath.Join(catalogPath, "terra-generate.hcl")

	src, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		output.Info("no catalog config found at %s", configPath)
		return &CatalogConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading catalog config %s: %w", configPath, err)
	}

	file, diags := hclsyntax.ParseConfig(src, configPath, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parsing catalog config %s: %s", configPath, diags.Error())
	}

	fileContent, _, diags := file.Body.PartialContent(catalogConfigFileSchema)
	if diags.HasErrors() {
		return nil, fmt.Errorf("reading catalog config %s: %s", configPath, diags.Error())
	}

	cfg := &CatalogConfig{}

	var configBlock *hcl.Block
	for _, block := range fileContent.Blocks {
		if block.Type == "config" {
			configBlock = block
			break
		}
	}
	if configBlock == nil {
		return cfg, nil
	}

	content, _, diags := configBlock.Body.PartialContent(catalogConfigBlockSchema)
	if diags.HasErrors() {
		return nil, fmt.Errorf("reading catalog config %s: %s", configPath, diags.Error())
	}

	if attr, ok := content.Attributes["ignore_deps"]; ok {
		val, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			return nil, fmt.Errorf("evaluating ignore_deps: %s", diags.Error())
		}
		if !val.Type().IsListType() && !val.Type().IsTupleType() {
			return nil, fmt.Errorf("ignore_deps must be a list of strings, got %s", val.Type().FriendlyName())
		}
		for it := val.ElementIterator(); it.Next(); {
			_, v := it.Element()
			if v.Type() != cty.String {
				return nil, fmt.Errorf("ignore_deps elements must be strings")
			}
			cfg.IgnoreDeps = append(cfg.IgnoreDeps, v.AsString())
		}
	}

	if attr, ok := content.Attributes["name_must_match"]; ok {
		val, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			return nil, fmt.Errorf("evaluating name_must_match: %s", diags.Error())
		}
		if val.Type() != cty.String {
			return nil, fmt.Errorf("name_must_match must be a string, got %s", val.Type().FriendlyName())
		}
		cfg.NameMustMatch = val.AsString()
	}

	output.Info("loaded catalog config from %s", configPath)
	return cfg, nil
}
