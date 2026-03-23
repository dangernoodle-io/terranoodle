package generator

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"dangernoodle.io/terra-tools/internal/catalog/catalog"
)

// InjectDependencies modifies generated terragrunt.hcl content to add dependency
// and dependencies blocks for cross-sub-tenant dependencies.
//
// It checks whether servicePath is listed as a target in any depends_on entry
// for the current template. For each match, it appends dependency blocks pointing
// to the parent template's source service.
//
// dependsOn structure: parentTemplate -> sourceService -> []targetServices
// If servicePath is in targetServices, add a dependency on parentTemplate/sourceService.
func InjectDependencies(content []byte, templateName string, dependsOn map[string]map[string][]string, servicePath string) ([]byte, error) {
	if len(dependsOn) == 0 {
		return content, nil
	}

	type depEntry struct {
		label      string // dependency block label (source service name, last path component)
		configPath string // relative config_path
	}

	var deps []depEntry

	for parentTemplate, serviceMap := range dependsOn {
		for sourceService, targetServices := range serviceMap {
			for _, target := range targetServices {
				if target == servicePath {
					relPath := computeRelPath(templateName, servicePath, parentTemplate, sourceService)
					label := filepath.Base(sourceService)
					deps = append(deps, depEntry{
						label:      label,
						configPath: relPath,
					})
					break
				}
			}
		}
	}

	if len(deps) == 0 {
		return content, nil
	}

	var sb strings.Builder

	// Append a newline separator if content doesn't end with one.
	if len(content) > 0 && content[len(content)-1] != '\n' {
		sb.WriteByte('\n')
	}
	sb.WriteByte('\n')

	// Write the dependencies block (lists all paths).
	sb.WriteString("dependencies {\n")
	sb.WriteString("  paths = [")
	for i, dep := range deps {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "%q", dep.configPath)
	}
	sb.WriteString("]\n")
	sb.WriteString("}\n")

	// Write individual dependency blocks.
	for _, dep := range deps {
		sb.WriteByte('\n')
		fmt.Fprintf(&sb, "dependency %q {\n", dep.label)
		fmt.Fprintf(&sb, "  config_path = %q\n", dep.configPath)
		sb.WriteString("}\n")
	}

	return append(content, []byte(sb.String())...), nil
}

// computeRelPath computes the relative path from the current template/service
// location to the parent template/source service location.
//
// All templates are siblings at the same level under the output directory:
//
//	<outputDir>/
//	  <templateName>/
//	    <servicePath>             e.g. cloud-run/
//	    region/<servicePath>      e.g. region/cloud-run/
//	  <parentTemplate>/
//	    <sourceService>/
//
// From <templateName>/<servicePath>/terragrunt.hcl to <parentTemplate>/<sourceService>:
//   - Each servicePath component adds one level of "../"
//   - Plus one extra "../" to escape <templateName>
func computeRelPath(_ string, servicePath, parentTemplate, sourceService string) string {
	// Count the depth of the service path (number of "/" separators + 1).
	depth := len(strings.Split(servicePath, "/"))

	// Go up (depth + 1) levels: depth levels for the service path + 1 for templateName.
	ups := strings.Repeat("../", depth+1)
	ups = strings.TrimSuffix(ups, "/") // remove trailing slash from last element

	return ups + "/" + parentTemplate + "/" + sourceService
}

// CrossTemplateDep represents a dependency that crosses template boundaries.
type CrossTemplateDep struct {
	SourceTemplate string // template containing the dependent service
	SourceService  string // service path that has the dependency (catalog path)
	TargetTemplate string // template containing the dependency target
	TargetService  string // service path of the dependency target (catalog path)
	Label          string // dependency block label
}

// ResolveCrossTemplateDeps determines which catalog dependencies are cross-template.
// It takes all templates with their resolved service sets and the catalog layout.
// Returns cross-template deps grouped by source template+service, and any errors.
func ResolveCrossTemplateDeps(
	templates []struct {
		Name     string
		Services map[string]bool // base names of services in this template's values
	},
	layout *catalog.Layout,
	ignoreDeps map[string]bool,
) ([]CrossTemplateDep, error) {
	// Build a lookup from base name to catalog service path.
	baseToPath := make(map[string]string)
	for path, svc := range layout.Services {
		baseName := path
		if svc.IsRegion {
			baseName = filepath.Base(path)
		}
		baseToPath[baseName] = path
	}

	// Build a lookup: service base name → list of template names that contain it.
	serviceToTemplates := make(map[string][]string)
	for _, tmpl := range templates {
		for baseName := range tmpl.Services {
			serviceToTemplates[baseName] = append(serviceToTemplates[baseName], tmpl.Name)
		}
	}
	// Sort template lists for deterministic error messages.
	for k := range serviceToTemplates {
		sort.Strings(serviceToTemplates[k])
	}

	var crossDeps []CrossTemplateDep

	for _, tmpl := range templates {
		for baseName := range tmpl.Services {
			// Look up catalog service path for this base name.
			svcPath, ok := baseToPath[baseName]
			if !ok {
				continue // not a catalog service (project-level value)
			}

			svc := layout.Services[svcPath]
			for _, depLabel := range svc.Dependencies {
				if ignoreDeps[depLabel] {
					continue
				}

				// Find which template(s) contain this dep label.
				ownerTemplates := serviceToTemplates[depLabel]

				if len(ownerTemplates) == 0 {
					return nil, fmt.Errorf(
						"template %q service %q: dependency %q is not present in any template",
						tmpl.Name, svcPath, depLabel,
					)
				}

				// Check if the dep is in the same template — intra-template, skip.
				inSame := false
				for _, owner := range ownerTemplates {
					if owner == tmpl.Name {
						inSame = true
						break
					}
				}
				if inSame {
					continue
				}

				// Filter out the source template itself (shouldn't be there, but defensive).
				var otherTemplates []string
				for _, owner := range ownerTemplates {
					if owner != tmpl.Name {
						otherTemplates = append(otherTemplates, owner)
					}
				}

				if len(otherTemplates) > 1 {
					return nil, fmt.Errorf(
						"template %q service %q: dependency %q is ambiguous — present in templates: %s",
						tmpl.Name, svcPath, depLabel, strings.Join(otherTemplates, ", "),
					)
				}

				targetTemplate := otherTemplates[0]
				targetPath, ok := baseToPath[depLabel]
				if !ok {
					return nil, fmt.Errorf(
						"template %q service %q: dependency %q does not map to a catalog service path",
						tmpl.Name, svcPath, depLabel,
					)
				}

				crossDeps = append(crossDeps, CrossTemplateDep{
					SourceTemplate: tmpl.Name,
					SourceService:  svcPath,
					TargetTemplate: targetTemplate,
					TargetService:  targetPath,
					Label:          depLabel,
				})
			}
		}
	}

	return crossDeps, nil
}

// StripDependencyBlocks removes dependency blocks with the given labels from
// rendered HCL content. Also removes any dependencies { paths = [...] } block
// if any of its entries correspond to stripped labels.
func StripDependencyBlocks(content []byte, labels map[string]bool) ([]byte, error) {
	if len(labels) == 0 {
		return content, nil
	}

	file, diags := hclsyntax.ParseConfig(content, "<strip>", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parsing HCL for dependency stripping: %s", diags.Error())
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("unexpected body type when stripping dependency blocks")
	}

	// Collect byte ranges to remove, in the order we find them.
	type byteRange struct {
		start int
		end   int
	}
	var ranges []byteRange

	for _, block := range body.Blocks {
		switch block.Type {
		case "dependency":
			if len(block.Labels) == 1 && labels[block.Labels[0]] {
				ranges = append(ranges, byteRange{
					start: block.DefRange().Start.Byte,
					end:   block.Body.EndRange.End.Byte,
				})
			}

		case "dependencies":
			// Strip the entire dependencies block if ANY of its path entries
			// match a stripped dep label. InjectDependencies will rebuild it.
			if dependenciesBlockMatchesLabels(block, labels) {
				ranges = append(ranges, byteRange{
					start: block.DefRange().Start.Byte,
					end:   block.Body.EndRange.End.Byte,
				})
			}
		}
	}

	if len(ranges) == 0 {
		return content, nil
	}

	// Sort ranges in reverse order by start byte so removal doesn't shift offsets.
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].start > ranges[j].start
	})

	result := make([]byte, len(content))
	copy(result, content)

	for _, r := range ranges {
		// Expand backwards over any leading whitespace/newline on the same removal line.
		start := r.start
		for start > 0 && (result[start-1] == ' ' || result[start-1] == '\t') {
			start--
		}
		// If what precedes the whitespace is a newline (or start of file), eat that newline too.
		if start > 0 && result[start-1] == '\n' {
			start--
		}

		end := r.end
		// Expand forwards over trailing newline following the closing brace.
		if end < len(result) && result[end] == '\n' {
			end++
		}

		result = append(result[:start], result[end:]...)
	}

	// Collapse runs of more than two consecutive newlines into two.
	result = collapseBlankLines(result)

	return result, nil
}

// dependenciesBlockMatchesLabels checks whether any path inside a dependencies
// block ends with a segment matching one of the given labels.
func dependenciesBlockMatchesLabels(block *hclsyntax.Block, labels map[string]bool) bool {
	for _, attr := range block.Body.Attributes {
		if attr.Name != "paths" {
			continue
		}
		tupleExpr, ok := attr.Expr.(*hclsyntax.TupleConsExpr)
		if !ok {
			continue
		}
		for _, elem := range tupleExpr.Exprs {
			val, diags := elem.Value(nil)
			if diags.HasErrors() {
				continue
			}
			if val.Type() == cty.String {
				pathStr := val.AsString()
				base := filepath.Base(pathStr)
				if labels[base] {
					return true
				}
			}
		}
	}
	return false
}

// collapseBlankLines replaces runs of 3+ consecutive newlines with 2 newlines.
func collapseBlankLines(content []byte) []byte {
	tripleNL := []byte("\n\n\n")
	doubleNL := []byte("\n\n")
	for bytes.Contains(content, tripleNL) {
		content = bytes.ReplaceAll(content, tripleNL, doubleNL)
	}
	return content
}
