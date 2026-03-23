package resolver

import (
	"context"
	"fmt"
	"os"
	"sort"

	tfjson "github.com/hashicorp/terraform-json"

	"dangernoodle.io/terratools/internal/state/config"
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
// in cfg. If cfg.API is set and the matched type references resolvers, each
// resolver is executed in dependency order via the API client before the ID
// template is rendered.
//
// varOverrides are merged on top of cfg.Vars (overrides win).
func Resolve(
	resources []*tfjson.ResourceChange,
	cfg *config.Config,
	varOverrides map[string]string,
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
	needsAPI := cfg.API != nil && len(cfg.Resolvers) > 0

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

	// Lazy-create the API client only if we actually need it.
	var client *APIClient

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

			if client == nil {
				token := os.Getenv(cfg.API.TokenEnv)
				client = NewAPIClient(cfg.API.BaseURL, token)
			}

			for _, name := range resolverOrder {
				if !required[name] {
					continue
				}

				res := cfg.Resolvers[name]

				getPath, err := RenderID(res.Get, rc, merged, resolverResults)
				if err != nil {
					return nil, fmt.Errorf("resolver: %s: resolver %q get: %w", rc.Address, name, err)
				}

				apiResp, err := client.Get(context.Background(), getPath)
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
// before dependents) using Kahn's algorithm. It returns an error only if a
// cycle is detected, which should already have been caught by config.Validate.
func topoSortResolvers(resolvers map[string]config.Resolver) ([]string, error) {
	// inDegree counts unmet dependencies for each resolver.
	inDegree := make(map[string]int, len(resolvers))
	// dependents[dep] = list of resolvers that depend on dep.
	dependents := make(map[string][]string, len(resolvers))

	for name := range resolvers {
		inDegree[name] = 0
	}

	for name, res := range resolvers {
		for _, dep := range res.Use {
			if _, ok := resolvers[dep]; !ok {
				continue // undefined ref; already caught by validator
			}
			inDegree[name]++
			dependents[dep] = append(dependents[dep], name)
		}
	}

	// Seed the queue with resolvers that have no dependencies.
	queue := make([]string, 0, len(resolvers))
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	// Deterministic output within the same dependency level.
	sort.Strings(queue)

	order := make([]string, 0, len(resolvers))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		// Gather dependents and sort for determinism before appending to queue.
		ready := make([]string, 0)
		for _, dependent := range dependents[node] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
		sort.Strings(ready)
		queue = append(queue, ready...)
	}

	if len(order) != len(resolvers) {
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
