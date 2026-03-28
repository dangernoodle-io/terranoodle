package scaffold

import (
	"embed"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/template"

	tfjson "github.com/hashicorp/terraform-json"

	profileconfig "dangernoodle.io/terranoodle/internal/config"
	"dangernoodle.io/terranoodle/internal/state/config"
	"dangernoodle.io/terranoodle/internal/state/fields"
)

//go:embed templates/scaffold.yml.tmpl
var scaffoldTemplateFS embed.FS

var scaffoldTmpl = template.Must(
	template.New("scaffold.yml.tmpl").
		ParseFS(scaffoldTemplateFS, "templates/scaffold.yml.tmpl"),
)

// TypeInfo holds scaffold data for a single resource type.
type TypeInfo struct {
	ResourceType string
	Fields       map[string]string // field name → sample value (from plan)
	IDTemplate   string            // "TODO" for now; registry lookup is a future phase
}

// Generate deduplicates resources by type, collects a sample field set from
// the first occurrence of each type, and returns a list sorted by type name.
// formats is an optional map of resource type → import format string fetched
// from the provider registry. Pass nil to fall back to "TODO" for all types.
func Generate(resources []*tfjson.ResourceChange, formats map[string]string) []TypeInfo {
	seen := make(map[string]*TypeInfo)
	var order []string

	for _, rc := range resources {
		if rc == nil || rc.Change == nil {
			continue
		}

		rt := rc.Type
		if _, exists := seen[rt]; exists {
			continue
		}

		info := &TypeInfo{
			ResourceType: rt,
			Fields:       make(map[string]string),
			IDTemplate:   "TODO",
		}

		// Collect field names and one sample value from Change.After.
		if after, ok := rc.Change.After.(map[string]interface{}); ok {
			// Extract string-representable fields.
			extracted := fields.ExtractStrings(after)
			for k, v := range extracted {
				info.Fields[k] = v
			}
			// Handle nil values separately (ExtractStrings skips them).
			for k, v := range after {
				if v == nil {
					info.Fields[k] = ""
				}
			}
		}

		// If a registry format was fetched for this type, convert it to a template.
		if formats != nil {
			if regFmt, ok := formats[rt]; ok && regFmt != "" {
				info.IDTemplate = FormatToTemplate(regFmt, info.Fields)
			}
		}

		seen[rt] = info
		order = append(order, rt)
	}

	sort.Strings(order)

	result := make([]TypeInfo, 0, len(order))
	for _, rt := range order {
		result = append(result, *seen[rt])
	}
	return result
}

type scaffoldTemplateData struct {
	ProviderGroups []scaffoldProviderGroup
}

type scaffoldProviderGroup struct {
	Provider string
	Types    []scaffoldTypeEntry
}

type scaffoldTypeEntry struct {
	ResourceType  string
	FieldsComment string
	IDTemplate    string
	HasFields     bool
}

// buildScaffoldData groups and formats types for the scaffold template.
func buildScaffoldData(types []TypeInfo) scaffoldTemplateData {
	providerOrder := []string{}
	byProvider := make(map[string][]TypeInfo)
	for _, ti := range types {
		p := ProviderFromType(ti.ResourceType)
		if _, exists := byProvider[p]; !exists {
			providerOrder = append(providerOrder, p)
		}
		byProvider[p] = append(byProvider[p], ti)
	}

	var data scaffoldTemplateData
	for _, provider := range providerOrder {
		group := byProvider[provider]
		pg := scaffoldProviderGroup{Provider: provider}

		for _, ti := range group {
			entry := scaffoldTypeEntry{
				ResourceType: ti.ResourceType,
				IDTemplate:   ti.IDTemplate,
				HasFields:    len(ti.Fields) > 0,
			}

			if len(ti.Fields) > 0 {
				fieldNames := make([]string, 0, len(ti.Fields))
				for k := range ti.Fields {
					fieldNames = append(fieldNames, k)
				}
				sort.Strings(fieldNames)

				parts := make([]string, 0, len(fieldNames))
				for _, k := range fieldNames {
					v := ti.Fields[k]
					if v == "" {
						parts = append(parts, fmt.Sprintf(".%s = (null)", k))
					} else {
						parts = append(parts, fmt.Sprintf(".%s = %q", k, v))
					}
				}
				entry.FieldsComment = strings.Join(parts, ", ")
			}

			pg.Types = append(pg.Types, entry)
		}

		data.ProviderGroups = append(data.ProviderGroups, pg)
	}

	return data
}

// RenderYAML writes a scaffold YAML config to w, grouped by provider.
func RenderYAML(w io.Writer, types []TypeInfo) error {
	if len(types) == 0 {
		_, err := fmt.Fprintln(w, "# No resource types found in plan.")
		return err
	}
	return scaffoldTmpl.Execute(w, buildScaffoldData(types))
}

// PreFill replaces "TODO" IDTemplates with known templates from central state files.
// loadFn is injectable for testing (in production pass store.Load).
// globalCfg may be nil (falls back to "default" state for all providers).
func PreFill(types []TypeInfo, globalCfg *profileconfig.GlobalConfig, statePathFn func(string) (string, error), loadFn func(string) (*config.Config, error)) []TypeInfo {
	// Cache loaded state configs keyed by state name
	stateCache := make(map[string]*config.Config)

	for i := range types {
		ti := &types[i]

		// Skip non-TODO templates
		if ti.IDTemplate != "TODO" {
			continue
		}

		// Get provider from resource type
		provider := ProviderFromType(ti.ResourceType)

		// Find profile for this provider
		profileName := profileconfig.ScaffoldProfileForProvider(globalCfg, provider)
		var stateName string

		// Get state name from profile, or use "default"
		if profileName != "" && globalCfg != nil && globalCfg.Profiles != nil {
			if profile, ok := globalCfg.Profiles[profileName]; ok {
				stateName = profile.Scaffold.State
			}
		}

		if stateName == "" {
			stateName = "default"
		}

		// Load state config if not cached
		var stateCfg *config.Config
		if cached, ok := stateCache[stateName]; ok {
			stateCfg = cached
		} else {
			path, err := statePathFn(stateName)
			if err != nil {
				continue
			}
			var err2 error
			stateCfg, err2 = loadFn(path)
			if err2 != nil {
				continue
			}
			stateCache[stateName] = stateCfg
		}

		// Look up type in state config
		if stateCfg != nil && stateCfg.Types != nil {
			if typeMapping, ok := stateCfg.Types[ti.ResourceType]; ok && typeMapping.ID != "" {
				ti.IDTemplate = typeMapping.ID
			}
		}
	}

	return types
}
