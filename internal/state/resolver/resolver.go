package resolver

import (
	"context"
	"fmt"
	"sort"

	tfjson "github.com/hashicorp/terraform-json"

	"dangernoodle.io/terranoodle/internal/state/config"
	"dangernoodle.io/terranoodle/internal/state/toposort"
)

// ImportEntry holds the resolved import address and ID for a single
// Terraform resource.
type ImportEntry struct {
	Address string
	ID      string
	Type    string
}

// Result contains the resources that were successfully resolved and those
// for which no type mapping was found.
type Result struct {
	Matched   []ImportEntry
	Unmatched []string
}

// Resolve maps each resource change to an import ID using the type mappings
// in cfg. If getter is non-nil and the matched type references resolvers, each
// resolver is executed in dependency order via the getter before the ID
// template is rendered.
//
// varOverrides are merged on top of cfg.Vars (overrides win).
// getter may be nil for configs without API resolvers.
func Resolve(
	resources []*tfjson.ResourceChange,
	cfg *config.Config,
	varOverrides map[string]string,
	getter Getter,
) (*Result, error) {
	// Merge vars: cfg.Vars is the base; varOverrides win on conflict.
	merged := make(map[string]string, len(cfg.Vars)+len(varOverrides))
	for k, v := range cfg.Vars {
		merged[k] = v
	}
	for k, v := range varOverrides {
		merged[k] = v
	}

	// Determine whether any resolver will be needed at all.
	needsAPI := getter != nil && len(cfg.Resolvers) > 0

	// Pre-compute the full execution order for resolvers (shared across all
	// resources). The per-resource resolver set is a subset of this order.
	var resolverOrder []string
	if needsAPI {
		var err error
		resolverOrder, err = topoSortResolvers(cfg.Resolvers)
		if err != nil {
			return nil, fmt.Errorf("resolver: %w", err)
		}
	}

	result := &Result{}

	for _, rc := range resources {
		tm, ok := cfg.Types[rc.Type]
		if !ok {
			result.Unmatched = append(result.Unmatched, rc.Address)
			continue
		}

		resolverResults := map[string]string{}

		if needsAPI && len(tm.Use) > 0 {
			required := transitiveResolvers(tm.Use, cfg.Resolvers)

			for _, name := range resolverOrder {
				if !required[name] {
					continue
				}

				res := cfg.Resolvers[name]

				getPath, err := RenderID(res.Get, rc, merged, resolverResults)
				if err != nil {
					return nil, fmt.Errorf("resolver: %s: resolver %q get: %w", rc.Address, name, err)
				}

				apiResp, err := getter.Get(context.Background(), getPath)
				if err != nil {
					return nil, fmt.Errorf("resolver: %s: resolver %q: %w", rc.Address, name, err)
				}

				ctx := BuildContext(rc, merged, resolverResults)
				picked, err := Pick(apiResp, res.Pick, ctx)
				if err != nil {
					return nil, fmt.Errorf("resolver: %s: resolver %q: %w", rc.Address, name, err)
				}

				resolverResults[name] = picked
			}
		}

		id, err := RenderID(tm.ID, rc, merged, resolverResults)
		if err != nil {
			return nil, fmt.Errorf("resolver: %s: %w", rc.Address, err)
		}

		result.Matched = append(result.Matched, ImportEntry{
			Address: rc.Address,
			ID:      id,
			Type:    rc.Type,
		})
	}

	sort.Slice(result.Matched, func(i, j int) bool {
		return result.Matched[i].Address < result.Matched[j].Address
	})

	return result, nil
}

// topoSortResolvers returns resolver names in execution order (dependencies
// before dependents) using topological sort. It returns an error only if a
// cycle is detected, which should already have been caught by config.Validate.
func topoSortResolvers(resolvers map[string]config.Resolver) ([]string, error) {
	// Build adjacency map: adjacency[resolver] = list of resolvers it depends on.
	adjacency := make(map[string][]string, len(resolvers))

	for name, res := range resolvers {
		var deps []string
		for _, dep := range res.Use {
			if _, ok := resolvers[dep]; !ok {
				continue // undefined ref; already caught by validator
			}
			deps = append(deps, dep)
		}
		adjacency[name] = deps
	}

	order, err := toposort.Sort(adjacency)
	if err != nil {
		return nil, fmt.Errorf("circular dependency detected among resolvers")
	}

	return order, nil
}

// transitiveResolvers returns the full set of resolver names (including
// transitive dependencies) required starting from the given roots.
func transitiveResolvers(roots []string, resolvers map[string]config.Resolver) map[string]bool {
	visited := make(map[string]bool)
	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		if res, ok := resolvers[name]; ok {
			for _, dep := range res.Use {
				visit(dep)
			}
		}
	}
	for _, r := range roots {
		visit(r)
	}
	return visited
}
