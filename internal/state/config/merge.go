package config

// Merge combines multiple configs in order. Later configs override earlier ones.
//   - Vars: merged, later wins
//   - Types: merged by key, later wins
//   - Resolvers: merged by key, later wins
//   - API: later non-nil wins
func Merge(configs ...*Config) *Config {
	out := &Config{
		Vars:      make(map[string]string),
		Types:     make(map[string]TypeMapping),
		Resolvers: make(map[string]Resolver),
	}

	for _, cfg := range configs {
		if cfg == nil {
			continue
		}

		for k, v := range cfg.Vars {
			out.Vars[k] = v
		}

		for k, v := range cfg.Types {
			out.Types[k] = v
		}

		for k, v := range cfg.Resolvers {
			out.Resolvers[k] = v
		}

		if cfg.API != nil {
			out.API = cfg.API
		}
	}

	return out
}
