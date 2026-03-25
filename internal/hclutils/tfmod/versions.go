package tfmod

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// ProviderDecl represents a provider declaration in required_providers.
type ProviderDecl struct {
	Name       string
	HasSource  bool
	HasVersion bool
	Version    string
	File       string
}

// VersionsResult holds the parsed state of versions.tf.
type VersionsResult struct {
	Exists            bool
	HasTerraformBlock bool
	Providers         []ProviderDecl
}

var versionsTerraformSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "terraform"},
	},
}

var requiredProvidersSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "required_providers"},
	},
}

// ParseVersionsTF parses versions.tf and scans all .tf files for provider declarations.
func ParseVersionsTF(moduleDir string) (*VersionsResult, error) {
	result := &VersionsResult{}

	versionsPath := filepath.Join(moduleDir, "versions.tf")
	if _, err := os.Stat(versionsPath); os.IsNotExist(err) {
		return result, nil
	}

	result.Exists = true

	src, err := os.ReadFile(versionsPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", versionsPath, err)
	}

	file, diags := hclsyntax.ParseConfig(src, versionsPath, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parsing %s: %s", versionsPath, diags.Error())
	}

	content, _, diags := file.Body.PartialContent(versionsTerraformSchema)
	if diags.HasErrors() {
		return nil, fmt.Errorf("decoding %s: %s", versionsPath, diags.Error())
	}

	// Check for terraform block
	for _, block := range content.Blocks {
		if block.Type == "terraform" {
			result.HasTerraformBlock = true
			// Parse required_providers inside terraform block
			rpContent, _, rpDiags := block.Body.PartialContent(requiredProvidersSchema)
			if rpDiags.HasErrors() {
				continue
			}
			for _, rpBlock := range rpContent.Blocks {
				if rpBlock.Type != "required_providers" {
					continue
				}
				// Each provider is an attribute in required_providers
				attrs, attrDiags := rpBlock.Body.JustAttributes()
				if attrDiags.HasErrors() {
					continue
				}
				for name, attr := range attrs {
					pd := ProviderDecl{
						Name: name,
						File: versionsPath,
					}
					// Evaluate the attribute to check for source and version
					val, valDiags := attr.Expr.Value(nil)
					if !valDiags.HasErrors() && val.IsKnown() && val.Type().IsObjectType() {
						if val.Type().HasAttribute("source") {
							sv := val.GetAttr("source")
							if sv.Type() == cty.String && sv.AsString() != "" {
								pd.HasSource = true
							}
						}
						if val.Type().HasAttribute("version") {
							sv := val.GetAttr("version")
							if sv.Type() == cty.String && sv.AsString() != "" {
								pd.HasVersion = true
								pd.Version = sv.AsString()
							}
						}
					}
					result.Providers = append(result.Providers, pd)
				}
			}
		}
	}

	// Scan ALL .tf files for duplicate provider declarations
	entries, err := os.ReadDir(moduleDir)
	if err != nil {
		return nil, fmt.Errorf("reading dir %s: %w", moduleDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".tf" || entry.Name() == "versions.tf" {
			continue
		}

		path := filepath.Join(moduleDir, entry.Name())
		otherSrc, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		otherFile, diags := hclsyntax.ParseConfig(otherSrc, path, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			continue
		}

		otherContent, _, otherDiags := otherFile.Body.PartialContent(versionsTerraformSchema)
		if otherDiags.HasErrors() {
			continue
		}

		for _, block := range otherContent.Blocks {
			if block.Type != "terraform" {
				continue
			}
			rpContent, _, rpDiags := block.Body.PartialContent(requiredProvidersSchema)
			if rpDiags.HasErrors() {
				continue
			}
			for _, rpBlock := range rpContent.Blocks {
				if rpBlock.Type != "required_providers" {
					continue
				}
				attrs, attrDiags := rpBlock.Body.JustAttributes()
				if attrDiags.HasErrors() {
					continue
				}
				for name := range attrs {
					result.Providers = append(result.Providers, ProviderDecl{
						Name: name,
						File: path,
					})
				}
			}
		}
	}

	return result, nil
}
