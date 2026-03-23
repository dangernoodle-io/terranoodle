package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/zclconf/go-cty/cty"

	"dangernoodle.io/terratools/internal/catalog/catalog"
	"dangernoodle.io/terratools/internal/catalog/hclparse"
	"dangernoodle.io/terratools/internal/output"
)

// Config holds all inputs needed for generation.
type Config struct {
	TemplateDef *hclparse.TemplateDef
	Catalog     *catalog.Layout
	OutputDir   string
	Scaffold    bool
	DryRun      bool
}

func scaffoldContent() []byte {
	return []byte(`include "root" {
  path = find_in_parent_folders("terragrunt-root.hcl")
}

exclude {
  if      = true
  actions = ["all"]
}
`)
}

// Error represents a generation/validation error.
type Error struct {
	Template string // template block name
	Service  string // service name (empty for template-level errors)
	Detail   string
}

func (e Error) Error() string {
	if e.Service != "" {
		return fmt.Sprintf("template %q service %q: %s", e.Template, e.Service, e.Detail)
	}
	return fmt.Sprintf("template %q: %s", e.Template, e.Detail)
}

// Generate validates the template definition against the catalog and writes output.
// Returns errors if validation fails (does not write partial output).
func Generate(cfg *Config) ([]Error, error) {
	def := cfg.TemplateDef
	layout := cfg.Catalog

	// Build a set of catalog service base names (top-level dir names without region/ prefix).
	// Used to distinguish project-level values from service values.
	serviceBaseNames := make(map[string]bool)
	for path := range layout.Services {
		parts := strings.SplitN(path, "/", 2)
		if parts[0] == "region" && len(parts) == 2 {
			// e.g. "region/redis" -> base name is "redis"
			serviceBaseNames[filepath.Base(path)] = true
		} else {
			serviceBaseNames[path] = true
		}
	}

	// --- Phase 1: Validate ---
	nameMustMatch := def.NameMustMatch
	if nameMustMatch == "" && layout.Config != nil {
		nameMustMatch = layout.Config.NameMustMatch
	}
	if errs := validate(def, layout, serviceBaseNames, nameMustMatch); len(errs) > 0 {
		return errs, nil
	}

	// --- Phase 1b: Validate service dependencies ---
	ignoreDeps := mergeIgnoreDeps(
		layout.Config.IgnoreDeps,
		def.IgnoreDeps,
	)

	var depErrors []Error
	for _, tmpl := range def.Stacks {
		depErrors = append(depErrors, validateServiceDeps(tmpl.Name, tmpl.Values, layout, ignoreDeps)...)
	}
	if len(depErrors) > 0 {
		return depErrors, nil
	}

	// Warn about unused catalog services (informational only).
	for _, unused := range warnUnusedServices(def.Stacks, serviceBaseNames) {
		output.Warn("WARNING: catalog service %q is not referenced by any template", unused)
	}

	// --- Phase 1c: Resolve cross-template dependencies ---
	// Build template service sets for cross-template resolution.
	var templateServiceSets []struct {
		Name     string
		Services map[string]bool
	}
	for _, tmpl := range def.Stacks {
		services := make(map[string]bool)
		for k := range tmpl.Values {
			if serviceBaseNames[k] {
				services[k] = true
			}
		}
		templateServiceSets = append(templateServiceSets, struct {
			Name     string
			Services map[string]bool
		}{Name: tmpl.Name, Services: services})
	}

	crossDeps, err := ResolveCrossTemplateDeps(templateServiceSets, layout, ignoreDeps)
	if err != nil {
		return nil, fmt.Errorf("resolving cross-template dependencies: %w", err)
	}

	// Index cross-template deps by source template + service path for lookup during write.
	type crossDepKey struct {
		template string
		service  string // catalog service path
	}
	crossDepsByService := make(map[crossDepKey][]CrossTemplateDep)
	for _, cd := range crossDeps {
		key := crossDepKey{template: cd.SourceTemplate, service: cd.SourceService}
		crossDepsByService[key] = append(crossDepsByService[key], cd)
	}

	// --- Phase 2: Resolve values (template.xxx cross-references) ---
	resolvedTemplates := make([]hclparse.UnitDef, len(def.Stacks))
	copy(resolvedTemplates, def.Stacks)

	for i := range resolvedTemplates {
		resolved, err := ResolveValues(&resolvedTemplates[i], resolvedTemplates)
		if err != nil {
			return nil, fmt.Errorf("resolving values for template %q: %w", resolvedTemplates[i].Name, err)
		}
		resolvedTemplates[i].Values = resolved
	}

	// --- Phase 3: Write output (or dry-run preview) ---
	if cfg.DryRun {
		var entries []dryEntry

		entries = append(entries, dryEntry{relPath: "terragrunt-root.hcl"})

		for _, tmpl := range resolvedTemplates {
			projectTemplatePath := filepath.Join(layout.ProjectDir, "terragrunt.hcl")
			if _, err := os.Stat(projectTemplatePath); err == nil {
				entries = append(entries, dryEntry{relPath: filepath.Join(tmpl.Name, "terragrunt.hcl")})
			}

			_, serviceValues := splitValues(tmpl.Values, serviceBaseNames)

			svcPaths := make([]string, 0, len(layout.Services))
			for sp := range layout.Services {
				svcPaths = append(svcPaths, sp)
			}
			sort.Strings(svcPaths)

			for _, svcPath := range svcPaths {
				svc := layout.Services[svcPath]
				valuesKey := svcPath
				if svc.IsRegion {
					valuesKey = filepath.Base(svcPath)
				}
				if _, ok := serviceValues[valuesKey]; !ok {
					if cfg.Scaffold {
						entries = append(entries, dryEntry{
							relPath:  filepath.Join(tmpl.Name, svcPath, "terragrunt.hcl"),
							scaffold: true,
						})
					}
					continue
				}
				entries = append(entries, dryEntry{relPath: filepath.Join(tmpl.Name, svcPath, "terragrunt.hcl")})
			}
		}

		printDryRunTree(cfg.OutputDir, entries)
		return nil, nil
	}

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output directory %s: %w", cfg.OutputDir, err)
	}

	// Copy root terragrunt config.
	rootConfigDst := filepath.Join(cfg.OutputDir, "terragrunt-root.hcl")
	if err := copyFile(layout.RootConfig, rootConfigDst); err != nil {
		return nil, fmt.Errorf("copying root config: %w", err)
	}

	// Process each template.
	for _, tmpl := range resolvedTemplates {
		templateDir := filepath.Join(cfg.OutputDir, tmpl.Name)
		if err := os.MkdirAll(templateDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating template directory %s: %w", templateDir, err)
		}

		// Split values into project-level and service-level.
		projectValues, serviceValues := splitValues(tmpl.Values, serviceBaseNames)

		// Process project/terragrunt.hcl template.
		projectTemplatePath := filepath.Join(layout.ProjectDir, "terragrunt.hcl")
		if _, err := os.Stat(projectTemplatePath); err == nil {
			projectCty := mapToCtyObject(projectValues)
			rendered, resolveWarnings, err := ResolveTemplate(projectTemplatePath, projectCty)
			if err != nil {
				return nil, fmt.Errorf("template %q: resolving project template: %w", tmpl.Name, err)
			}
			for _, w := range resolveWarnings {
				output.Warn("WARNING: %s", w)
			}
			dstPath := filepath.Join(templateDir, "terragrunt.hcl")
			if err := os.WriteFile(dstPath, rendered, 0o644); err != nil {
				return nil, fmt.Errorf("template %q: writing project terragrunt.hcl: %w", tmpl.Name, err)
			}
		}

		// Process each catalog service.
		for svcPath, svc := range layout.Services {
			// Determine the values key for this service.
			// Regional services (region/redis) use base name "redis" as values key.
			valuesKey := svcPath
			if svc.IsRegion {
				valuesKey = filepath.Base(svcPath)
			}

			svcVals, ok := serviceValues[valuesKey]
			if !ok {
				if cfg.Scaffold {
					dstDir := filepath.Join(templateDir, svcPath)
					if err := os.MkdirAll(dstDir, 0o755); err != nil {
						return nil, fmt.Errorf("creating scaffold directory %s: %w", dstDir, err)
					}
					dstPath := filepath.Join(dstDir, "terragrunt.hcl")
					if err := os.WriteFile(dstPath, scaffoldContent(), 0o644); err != nil {
						return nil, fmt.Errorf("template %q service %q: writing scaffold: %w", tmpl.Name, svcPath, err)
					}
				}
				continue
			}

			// Merge project-level values with service-specific values.
			// This allows service templates to access project values like
			// tenant_friendly_id via values.xxx. Service values take precedence.
			mergedVals := mergeValues(projectValues, svcVals)

			// Resolve the service template.
			templatePath := filepath.Join(layout.ProjectDir, svcPath, "terragrunt.hcl")
			rendered, resolveWarnings, err := ResolveTemplate(templatePath, mergedVals)
			if err != nil {
				return nil, fmt.Errorf("template %q service %q: resolving template: %w", tmpl.Name, svcPath, err)
			}
			for _, w := range resolveWarnings {
				output.Warn("WARNING: %s", w)
			}

			// Handle cross-template dependencies.
			key := crossDepKey{template: tmpl.Name, service: svcPath}
			if ctDeps, ok := crossDepsByService[key]; ok {
				// Strip catalog dependency blocks that need cross-template rewriting.
				stripLabels := make(map[string]bool)
				for _, cd := range ctDeps {
					stripLabels[cd.Label] = true
				}
				rendered, err = StripDependencyBlocks(rendered, stripLabels)
				if err != nil {
					return nil, fmt.Errorf("template %q service %q: stripping dependency blocks: %w", tmpl.Name, svcPath, err)
				}

				// Build depends_on map for InjectDependencies (reuse existing function).
				dependsOn := make(map[string]map[string][]string)
				for _, cd := range ctDeps {
					if dependsOn[cd.TargetTemplate] == nil {
						dependsOn[cd.TargetTemplate] = make(map[string][]string)
					}
					dependsOn[cd.TargetTemplate][cd.TargetService] = append(
						dependsOn[cd.TargetTemplate][cd.TargetService], svcPath)
				}
				rendered, err = InjectDependencies(rendered, tmpl.Name, dependsOn, svcPath)
				if err != nil {
					return nil, fmt.Errorf("template %q service %q: injecting cross-template dependencies: %w", tmpl.Name, svcPath, err)
				}
			}

			// Write the rendered file.
			dstDir := filepath.Join(templateDir, svcPath)
			if err := os.MkdirAll(dstDir, 0o755); err != nil {
				return nil, fmt.Errorf("creating service directory %s: %w", dstDir, err)
			}
			dstPath := filepath.Join(dstDir, "terragrunt.hcl")
			if err := os.WriteFile(dstPath, rendered, 0o644); err != nil {
				return nil, fmt.Errorf("template %q service %q: writing terragrunt.hcl: %w", tmpl.Name, svcPath, err)
			}
		}
	}

	return nil, nil
}

type dryEntry struct {
	relPath  string
	scaffold bool
}

type treeNode struct {
	name     string
	children map[string]*treeNode
	file     *dryEntry
}

func newTreeNode(name string) *treeNode {
	return &treeNode{name: name, children: make(map[string]*treeNode)}
}

func insertPath(root *treeNode, entry dryEntry) {
	parts := strings.Split(filepath.ToSlash(entry.relPath), "/")
	cur := root
	for i, part := range parts {
		child, ok := cur.children[part]
		if !ok {
			child = newTreeNode(part)
			cur.children[part] = child
		}
		if i == len(parts)-1 {
			e := entry
			child.file = &e
		}
		cur = child
	}
}

func sortedChildren(n *treeNode) []*treeNode {
	keys := make([]string, 0, len(n.children))
	for k := range n.children {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	nodes := make([]*treeNode, 0, len(keys))
	for _, k := range keys {
		nodes = append(nodes, n.children[k])
	}
	return nodes
}

var dimColor = color.New(color.FgHiBlack)

func printTree(n *treeNode, prefix string, isLast bool, isRoot bool) {
	children := sortedChildren(n)
	isDir := len(children) > 0

	if !isRoot {
		connector := "├─ "
		if isLast {
			connector = "└─ "
		}
		label := n.name
		if isDir {
			label += "/"
		}
		line := prefix + connector + label
		if n.file != nil && n.file.scaffold {
			output.Info("%s", dimColor.Sprintf("%s [excluded]", line))
		} else {
			output.Info("%s", line)
		}
	}

	childPrefix := prefix
	if !isRoot {
		if isLast {
			childPrefix += "  "
		} else {
			childPrefix += "│ "
		}
	}

	for i, child := range children {
		printTree(child, childPrefix, i == len(children)-1, false)
	}
}

func printDryRunTree(outputDir string, entries []dryEntry) {
	root := newTreeNode(outputDir)
	for _, e := range entries {
		insertPath(root, e)
	}

	output.Info("Dry run — %s/", outputDir)
	children := sortedChildren(root)
	for i, child := range children {
		printTree(child, "", i == len(children)-1, false)
	}

	normal := 0
	scaffold := 0
	for _, e := range entries {
		if e.scaffold {
			scaffold++
		} else {
			normal++
		}
	}

	if scaffold > 0 {
		output.Info("%d file(s) would be written, %d excluded (scaffold).", normal, scaffold)
	} else {
		output.Info("%d file(s) would be written.", normal)
	}
}

// validate checks the template definition for internal consistency:
//   - values keys that look like service names (contain hyphens) but don't match
//     any catalog service are warned about
//
// It returns a slice of Error (possibly nil) and no fatal error.
func validate(def *hclparse.TemplateDef, _ *catalog.Layout, serviceBaseNames map[string]bool, nameMustMatch string) []Error {
	var errs []Error

	for _, tmpl := range def.Stacks {
		// Validate template name matches the configured values key (if set).
		if nameMustMatch != "" {
			matchKey := nameMustMatch
			if val, ok := tmpl.Values[matchKey]; ok {
				if val.Type() == cty.String && val.IsKnown() {
					if val.AsString() != tmpl.Name {
						errs = append(errs, Error{
							Template: tmpl.Name,
							Detail:   fmt.Sprintf("template name %q does not match %s %q", tmpl.Name, matchKey, val.AsString()),
						})
					}
				}
			} else {
				errs = append(errs, Error{
					Template: tmpl.Name,
					Detail:   fmt.Sprintf("template name %q requires values key %q (name_must_match) but it is missing", tmpl.Name, matchKey),
				})
			}
		}

		// Validation: warn about values keys that look like service names but
		// don't match any catalog service base name.
		allValueKeys := make(map[string]bool)
		for k := range tmpl.Values {
			allValueKeys[k] = true
		}
		for k := range tmpl.RawValues {
			allValueKeys[k] = true
		}
		for key := range allValueKeys {
			if strings.Contains(key, "-") && !serviceBaseNames[key] {
				errs = append(errs, Error{
					Template: tmpl.Name,
					Detail:   fmt.Sprintf("warning: values key %q contains hyphens but does not match any catalog service; it will be treated as a project value", key),
				})
			}
		}
	}

	return errs
}

// warnUnusedServices returns the names of catalog services that are not referenced
// by any template's values or rawValues keys.
func warnUnusedServices(templates []hclparse.UnitDef, serviceBaseNames map[string]bool) []string {
	used := make(map[string]bool)
	for _, tmpl := range templates {
		for key := range tmpl.Values {
			used[key] = true
		}
		for key := range tmpl.RawValues {
			used[key] = true
		}
	}

	var unused []string
	for name := range serviceBaseNames {
		if !used[name] {
			unused = append(unused, name)
		}
	}
	sort.Strings(unused)
	return unused
}

// splitValues separates a template's values map into project-level values (keys that
// don't match any catalog service base name) and service values (keys that do).
func splitValues(values map[string]cty.Value, serviceBaseNames map[string]bool) (map[string]cty.Value, map[string]cty.Value) {
	project := make(map[string]cty.Value)
	services := make(map[string]cty.Value)

	for k, v := range values {
		if serviceBaseNames[k] {
			services[k] = v
		} else {
			project[k] = v
		}
	}

	return project, services
}

// mapToCtyObject converts a map[string]cty.Value to a cty.Value object.
// Returns cty.EmptyObjectVal if the map is empty.
func mapToCtyObject(m map[string]cty.Value) cty.Value {
	if len(m) == 0 {
		return cty.EmptyObjectVal
	}
	return cty.ObjectVal(m)
}

// mergeValues merges project-level values with service-specific values.
// If svcVals is an object type, its attributes are merged on top of project values.
// If svcVals is not an object (e.g., empty object or non-object), project values
// are returned with the service values ignored at the top level.
func mergeValues(projectValues map[string]cty.Value, svcVals cty.Value) cty.Value {
	merged := make(map[string]cty.Value, len(projectValues))
	for k, v := range projectValues {
		merged[k] = v
	}

	if svcVals.Type().IsObjectType() {
		for name := range svcVals.Type().AttributeTypes() {
			merged[name] = svcVals.GetAttr(name)
		}
	}

	if len(merged) == 0 {
		return cty.EmptyObjectVal
	}
	return cty.ObjectVal(merged)
}

// mergeIgnoreDeps returns the union of catalog-level and template-level
// ignore_deps labels as a set.
func mergeIgnoreDeps(catalogDeps, templateDeps []string) map[string]bool {
	merged := make(map[string]bool)
	for _, d := range catalogDeps {
		merged[d] = true
	}
	for _, d := range templateDeps {
		merged[d] = true
	}
	return merged
}

// validateServiceDeps checks that all services in values have their catalog
// dependencies satisfied within the same values map. It walks dependencies
// transitively (BFS) and returns validation errors for any missing deps.
func validateServiceDeps(templateName string, values map[string]cty.Value, layout *catalog.Layout, ignoreDeps map[string]bool) []Error {
	// Build a lookup from base name to catalog service path.
	baseToPath := make(map[string]string)
	for path, svc := range layout.Services {
		baseName := path
		if svc.IsRegion {
			baseName = filepath.Base(path)
		}
		baseToPath[baseName] = path
	}

	// Seed the queue from services present in values.
	visited := make(map[string]bool)
	queue := make([]string, 0, len(values))
	for k := range values {
		if _, ok := baseToPath[k]; ok {
			queue = append(queue, k)
		}
	}

	var errs []Error

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		svcPath := baseToPath[current]
		svc := layout.Services[svcPath]

		for _, depLabel := range svc.Dependencies {
			if ignoreDeps[depLabel] {
				continue
			}

			if _, inValues := values[depLabel]; inValues {
				// Dep is present in values — enqueue for transitive walk.
				if !visited[depLabel] {
					queue = append(queue, depLabel)
				}
				continue
			}

			if _, isCatalog := baseToPath[depLabel]; isCatalog {
				// Dep is a known catalog service but not in values.
				errs = append(errs, Error{
					Template: templateName,
					Service:  current,
					Detail:   fmt.Sprintf("depends on %q (from catalog) but %q is not in template values", depLabel, depLabel),
				})
			} else {
				// Dep is neither a catalog service nor a structural dep.
				errs = append(errs, Error{
					Template: templateName,
					Service:  current,
					Detail:   fmt.Sprintf("depends on %q which is not a catalog service and not in ignore_deps", depLabel),
				})
			}
		}
	}

	return errs
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading %s: %w", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", dst, err)
	}
	return nil
}
