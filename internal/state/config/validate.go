package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/itchyny/gojq"
)

// Validate checks that cfg is internally consistent and returns a combined
// error listing all problems found, or nil when the config is valid.
func Validate(cfg *Config) error {
	var errs []string

	// At least one type must be declared.
	if len(cfg.Types) == 0 {
		errs = append(errs, "at least one type mapping is required")
	}

	// Track whether any resolver is actually used (by a type or another
	// resolver) so we can enforce the api requirement accurately.
	usesResolvers := false

	// Validate each type mapping.
	for name, tm := range cfg.Types {
		if strings.TrimSpace(tm.ID) == "" {
			errs = append(errs, fmt.Sprintf("type %q: id must not be empty", name))
		}
		for _, ref := range tm.Use {
			if _, ok := cfg.Resolvers[ref]; !ok {
				errs = append(errs, fmt.Sprintf("type %q: use references undefined resolver %q", name, ref))
			}
			usesResolvers = true
		}
	}

	// Validate each resolver.
	for name, res := range cfg.Resolvers {
		if strings.TrimSpace(res.Get) == "" {
			errs = append(errs, fmt.Sprintf("resolver %q: get must not be empty", name))
		}
		if res.Pick == nil {
			errs = append(errs, fmt.Sprintf("resolver %q: pick must not be empty", name))
		} else {
			_, _, jqExpr, parseErr := ParsePick(res.Pick)
			if parseErr != nil {
				errs = append(errs, fmt.Sprintf("resolver %q: invalid pick: %v", name, parseErr))
			} else if jqExpr != "" {
				if _, jqErr := gojq.Parse(jqExpr); jqErr != nil {
					errs = append(errs, fmt.Sprintf("resolver %q: invalid jq expression %q: %v", name, jqExpr, jqErr))
				}
			}
		}
		for _, ref := range res.Use {
			if _, ok := cfg.Resolvers[ref]; !ok {
				errs = append(errs, fmt.Sprintf("resolver %q: use references undefined resolver %q", name, ref))
			}
			usesResolvers = true
		}
	}

	// Check for circular resolver dependencies.
	if cycleErr := checkResolverCycles(cfg.Resolvers); cycleErr != nil {
		errs = append(errs, cycleErr.Error())
	}

	// If resolvers are used anywhere, the api block is required.
	if usesResolvers {
		if cfg.API == nil {
			errs = append(errs, "api block is required when resolvers are used")
		} else {
			if strings.TrimSpace(cfg.API.BaseURL) == "" && strings.TrimSpace(cfg.API.OpenAPISpec) == "" {
				errs = append(errs, "api.base_url or api.openapi_spec must be set when resolvers are used")
			}
			if strings.TrimSpace(cfg.API.TokenEnv) == "" {
				errs = append(errs, "api.token_env must not be empty when resolvers are used")
			}
		}
	}

	if len(errs) > 0 {
		return errors.New("config validation errors:\n  - " + strings.Join(errs, "\n  - "))
	}
	return nil
}

// checkResolverCycles performs a topological sort (Kahn's algorithm) over the
// resolver dependency graph and returns an error if a cycle is detected.
func checkResolverCycles(resolvers map[string]Resolver) error {
	// Build adjacency list and in-degree map.
	// An edge A -> B means resolver A depends on (uses) resolver B.
	inDegree := make(map[string]int, len(resolvers))
	deps := make(map[string][]string, len(resolvers))

	for name := range resolvers {
		inDegree[name] = 0
	}

	for name, res := range resolvers {
		for _, dep := range res.Use {
			// Only track edges between known resolvers; undefined refs are
			// already reported by the main validator.
			if _, ok := resolvers[dep]; !ok {
				continue
			}
			deps[name] = append(deps[name], dep)
			inDegree[dep]++
		}
	}

	// Collect nodes with zero in-degree as the starting set.
	queue := make([]string, 0, len(resolvers))
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++

		for _, dep := range deps[node] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if visited != len(resolvers) {
		// Identify the nodes involved in cycles.
		var cycle []string
		for name, deg := range inDegree {
			if deg > 0 {
				cycle = append(cycle, name)
			}
		}
		return fmt.Errorf("circular dependency detected among resolvers: %s", strings.Join(cycle, ", "))
	}

	return nil
}
