package store

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	profileconfig "dangernoodle.io/terranoodle/internal/config"
	stateconfig "dangernoodle.io/terranoodle/internal/state/config"
	"dangernoodle.io/terranoodle/internal/state/scaffold"
)

// StatePath returns the state file path for a given state name.
// It expands to ~/.config/terranoodle/scaffold/state/<name>.yml.
func StatePath(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("store: get home dir: %w", err)
	}
	return filepath.Join(home, ".config", "terranoodle", "scaffold", "state", name+".yml"), nil
}

// Load reads a state file at the given path. If the file doesn't exist,
// it returns an empty non-nil Config with initialized maps.
// Otherwise, it delegates to stateconfig.Load.
func Load(path string) (*stateconfig.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &stateconfig.Config{
				Vars:      make(map[string]string),
				Types:     make(map[string]stateconfig.TypeMapping),
				Resolvers: make(map[string]stateconfig.Resolver),
			}, nil
		}
		return nil, fmt.Errorf("store: read %q: %w", path, err)
	}

	var cfg stateconfig.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("store: unmarshal %q: %w", path, err)
	}

	// Ensure maps are initialized
	if cfg.Vars == nil {
		cfg.Vars = make(map[string]string)
	}
	if cfg.Types == nil {
		cfg.Types = make(map[string]stateconfig.TypeMapping)
	}
	if cfg.Resolvers == nil {
		cfg.Resolvers = make(map[string]stateconfig.Resolver)
	}

	return &cfg, nil
}

// Save merges the incoming config with any existing state file, then writes
// the merged result to disk. New types override existing ones.
func Save(path string, incoming *stateconfig.Config) error {
	if incoming == nil {
		return fmt.Errorf("store: incoming config is nil")
	}

	// Load existing state (tolerant — empty if missing)
	existing, err := Load(path)
	if err != nil {
		return fmt.Errorf("store: load existing: %w", err)
	}

	// Merge: incoming wins on overlaps
	merged := stateconfig.Merge(existing, incoming)

	// Create directory
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("store: mkdir %q: %w", dir, err)
	}

	// Marshal and write
	data, err := yaml.Marshal(merged)
	if err != nil {
		return fmt.Errorf("store: marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("store: write %q: %w", path, err)
	}

	return nil
}

// PromptStateFile prompts the user for a state file name.
// Reads from r and writes to w.
// If user enters empty string, returns defaultName.
// Otherwise returns trimmed input.
func PromptStateFile(r io.Reader, w io.Writer, provider, defaultName string) (string, error) {
	prompt := fmt.Sprintf("Provider %q has no scaffold state mapping. Save to [%s]: ", provider, defaultName)
	if _, err := fmt.Fprint(w, prompt); err != nil {
		return "", fmt.Errorf("store: write prompt: %w", err)
	}

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("store: scan input: %w", err)
		}
		return defaultName, nil
	}

	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return defaultName, nil
	}

	return input, nil
}

// SaveTypes saves types grouped by provider to their respective state files.
// For each provider:
//   - Skips types with IDTemplate == "TODO"
//   - Looks up the profile via ScaffoldProfileForProvider
//   - If no profile match, prompts the user
//   - Saves the batch to the target state file
func SaveTypes(types []scaffold.TypeInfo, globalCfg *profileconfig.GlobalConfig, cwd string, r io.Reader, w io.Writer) error {
	if len(types) == 0 {
		return nil
	}

	// Group types by provider
	byProvider := make(map[string][]scaffold.TypeInfo)
	providerOrder := []string{}

	for _, ti := range types {
		if ti.IDTemplate == "TODO" {
			continue // Skip TODO types
		}

		provider := scaffold.ProviderFromType(ti.ResourceType)
		if _, exists := byProvider[provider]; !exists {
			providerOrder = append(providerOrder, provider)
		}
		byProvider[provider] = append(byProvider[provider], ti)
	}

	// Process each provider
	for _, provider := range providerOrder {
		providerTypes := byProvider[provider]

		// Find target profile
		profileName := profileconfig.ScaffoldProfileForProvider(globalCfg, provider)
		if profileName == "" {
			// Prompt for state file name
			stateName, err := PromptStateFile(r, w, provider, "default")
			if err != nil {
				return fmt.Errorf("store: prompt state file: %w", err)
			}
			// Save with the provided state name
			statePath, err := StatePath(stateName)
			if err != nil {
				return fmt.Errorf("store: stat path: %w", err)
			}

			batch := &stateconfig.Config{
				Vars:      make(map[string]string),
				Types:     make(map[string]stateconfig.TypeMapping),
				Resolvers: make(map[string]stateconfig.Resolver),
			}

			for _, ti := range providerTypes {
				batch.Types[ti.ResourceType] = stateconfig.TypeMapping{
					ID: ti.IDTemplate,
				}
			}

			if err := Save(statePath, batch); err != nil {
				return fmt.Errorf("store: save batch for %q: %w", provider, err)
			}
		} else {
			// Use profile's state name
			profile := globalCfg.Profiles[profileName]
			stateName := profile.Scaffold.State
			if stateName == "" {
				stateName = "default"
			}

			statePath, err := StatePath(stateName)
			if err != nil {
				return fmt.Errorf("store: stat path: %w", err)
			}

			batch := &stateconfig.Config{
				Vars:      make(map[string]string),
				Types:     make(map[string]stateconfig.TypeMapping),
				Resolvers: make(map[string]stateconfig.Resolver),
			}

			for _, ti := range providerTypes {
				batch.Types[ti.ResourceType] = stateconfig.TypeMapping{
					ID: ti.IDTemplate,
				}
			}

			if err := Save(statePath, batch); err != nil {
				return fmt.Errorf("store: save batch for %q: %w", provider, err)
			}
		}
	}

	return nil
}
