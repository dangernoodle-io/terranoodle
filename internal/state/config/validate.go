package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/itchyny/gojq"

	"dangernoodle.io/terranoodle/internal/state/toposort"
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

// checkResolverCycles uses topological sort to detect circular dependencies.
func checkResolverCycles(resolvers map[string]Resolver) error {
	// Build adjacency map: adjacency[resolver] = list of resolvers it depends on.
	adjacency := make(map[string][]string, len(resolvers))

	for name, res := range resolvers {
		var deps []string
		for _, dep := range res.Use {
			// Only track edges between known resolvers; undefined refs are
			// already reported by the main validator.
			if _, ok := resolvers[dep]; !ok {
				continue
			}
			deps = append(deps, dep)
		}
		adjacency[name] = deps
	}

	// Run topological sort; if it fails, a cycle was detected.
	_, err := toposort.Sort(adjacency)
	if err != nil {
		return fmt.Errorf("circular dependency detected among resolvers: %w", err)
	}

	return nil
}
