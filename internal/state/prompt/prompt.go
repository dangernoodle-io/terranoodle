package prompt

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"dangernoodle.io/terranoodle/internal/state/config"
	"dangernoodle.io/terranoodle/internal/state/resolver"
)

// ResolverResult holds the definition of a new resolver and its associated
// type mapping, built interactively through the APIAssisted flow.
type ResolverResult struct {
	Name        string
	Get         string
	Pick        interface{} // string (field name) or map[string]interface{} for where/field
	Use         []string
	TypeMapping config.TypeMapping
}

// ManualID prompts the user to supply an import ID for a resource that had no
// type mapping.  It prints the available plan fields so the user can construct
// a valid ID, then optionally asks whether to persist the answer as a template.
//
// Returns (id, save, nil) on success.  id is empty when the user chose to skip.
func ManualID(r io.Reader, w io.Writer, address string, resourceType string, fields map[string]string) (id string, save bool, err error) {
	scanner := bufio.NewScanner(r)
	return manualIDWithScanner(scanner, w, address, resourceType, fields)
}

// APIAssisted guides the user through an interactive flow to either enter an
// import ID manually or build a resolver definition for a resource type.
//
// Returns:
//   - id: the resolved import ID (may be empty if user skipped)
//   - save: whether the user wants to persist the result
//   - resolverDef: non-nil when the user built a resolver (choice [2])
//   - err: any read/network error
func APIAssisted(
	r io.Reader,
	w io.Writer,
	address string,
	resourceType string,
	fields map[string]string,
	getter resolver.Getter,
	baseURL string,
	existingResolvers []string,
	vars map[string]string,
) (id string, save bool, resolverDef *ResolverResult, err error) {
	scanner := bufio.NewScanner(r)

	// Step 1: Print context and present choices.
	fmt.Fprintf(w, "\nUnmatched: %s\n", address)
	fmt.Fprintf(w, "Type: %s\n", resourceType)
	if len(fields) > 0 {
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf(".%s = %q", k, fields[k]))
		}
		fmt.Fprintf(w, "Available fields: %s\n", strings.Join(parts, ", "))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "[1] Enter import ID manually")
	fmt.Fprintln(w, "[2] Build a resolver")
	fmt.Fprint(w, "Choice: ")

	choice, choiceErr := scanLine(scanner)
	if choiceErr != nil {
		return "", false, nil, choiceErr
	}
	if choice == "" || choice == "skip" {
		return "", false, nil, nil
	}

	if choice == "1" {
		// Delegate to ManualID using the same scanner's underlying reader.
		// We have already consumed the choice line; ManualID needs its own
		// scanner over r. Since scanner may have buffered data, we pass r
		// directly and re-create via ManualID's own scanner.
		manualID, manualSave, manualErr := manualIDWithScanner(scanner, w, address, resourceType, fields)
		return manualID, manualSave, nil, manualErr
	}

	if choice != "2" {
		fmt.Fprintf(w, "Unknown choice %q — skipping\n", choice)
		return "", false, nil, nil
	}

	// --- Step 2: Resolver name ---
	fmt.Fprint(w, "Resolver name (e.g., badge_id): ")
	resolverName, err := scanLine(scanner)
	if err != nil {
		return "", false, nil, err
	}
	if resolverName == "" {
		fmt.Fprintln(w, "No resolver name provided — skipping")
		return "", false, nil, nil
	}

	// --- Step 3: Dependencies ---
	fmt.Fprintln(w, "Use existing resolvers? (comma-separated, or empty for none)")
	if len(existingResolvers) > 0 {
		fmt.Fprintf(w, "Available: %s\n", strings.Join(existingResolvers, ", "))
	}
	fmt.Fprint(w, "> ")
	depsLine, err := scanLine(scanner)
	if err != nil {
		return "", false, nil, err
	}
	var deps []string
	for _, d := range strings.Split(depsLine, ",") {
		d = strings.TrimSpace(d)
		if d != "" {
			deps = append(deps, d)
		}
	}

	// --- Step 4: API endpoint ---
	fmt.Fprintf(w, "Base URL: %s\n", baseURL)
	fmt.Fprintln(w, "Enter GET path (use .field for plan values, $var for vars/resolvers):")
	fmt.Fprint(w, "> ")
	getPath, err := scanLine(scanner)
	if err != nil {
		return "", false, nil, err
	}
	if getPath == "" {
		fmt.Fprintln(w, "No GET path provided — skipping")
		return "", false, nil, nil
	}

	// --- Step 5: Test the endpoint ---
	var apiResp interface{}
	for {
		renderedPath := renderSimple(getPath, fields, vars)
		fmt.Fprintf(w, "Testing: GET %s%s\n", baseURL, renderedPath)

		var apiErr error
		apiResp, apiErr = getter.Get(context.Background(), renderedPath)
		if apiErr != nil {
			fmt.Fprintf(w, "Error: %v\n", apiErr)
			fmt.Fprintln(w, "Retry (r), enter new path (n), or skip (s)?")
			fmt.Fprint(w, "> ")
			retryChoice, scanErr := scanLine(scanner)
			if scanErr != nil {
				return "", false, nil, scanErr
			}
			switch strings.ToLower(retryChoice) {
			case "r", "retry", "":
				continue
			case "n", "new":
				fmt.Fprintln(w, "Enter new GET path:")
				fmt.Fprint(w, "> ")
				getPath, scanErr = scanLine(scanner)
				if scanErr != nil {
					return "", false, nil, scanErr
				}
				continue
			default:
				// skip
				return "", false, nil, nil
			}
		}
		break
	}

	// Display response summary.
	displayJSONSummary(w, apiResp)

	// --- Step 6: Pick strategy ---
	var pick interface{}
	switch v := apiResp.(type) {
	case map[string]interface{}:
		// Object response.
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintf(w, "Response is a JSON object with fields: %s\n", strings.Join(keys, ", "))
		fmt.Fprintln(w, "Which field contains the value you need?")
		fmt.Fprint(w, "> ")
		pickField, scanErr := scanLine(scanner)
		if scanErr != nil {
			return "", false, nil, scanErr
		}
		if pickField == "" {
			fmt.Fprintln(w, "No field provided — skipping")
			return "", false, nil, nil
		}
		pick = pickField

	case []interface{}:
		// Array response.
		fmt.Fprintf(w, "Response is a JSON array (%d elements)\n", len(v))
		if len(v) > 0 {
			if elem, ok := v[0].(map[string]interface{}); ok {
				keys := make([]string, 0, len(elem))
				for k := range elem {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				pairs := make([]string, 0, len(keys))
				for _, k := range keys {
					val := elem[k]
					pairs = append(pairs, fmt.Sprintf("%q: %v", k, val))
				}
				fmt.Fprintf(w, "Sample element: {%s}\n", strings.Join(pairs, ", "))
			}
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Which field to match by?")
		fmt.Fprint(w, "> ")
		whereField, scanErr := scanLine(scanner)
		if scanErr != nil {
			return "", false, nil, scanErr
		}
		fmt.Fprintln(w, "Match value (use .field for plan values):")
		fmt.Fprint(w, "> ")
		whereValue, scanErr := scanLine(scanner)
		if scanErr != nil {
			return "", false, nil, scanErr
		}
		fmt.Fprintln(w, "Which field to extract?")
		fmt.Fprint(w, "> ")
		extractField, scanErr := scanLine(scanner)
		if scanErr != nil {
			return "", false, nil, scanErr
		}
		if whereField == "" || extractField == "" {
			fmt.Fprintln(w, "Incomplete pick expression — skipping")
			return "", false, nil, nil
		}
		pick = map[string]interface{}{
			"where": map[string]string{whereField: whereValue},
			"field": extractField,
		}

	default:
		fmt.Fprintf(w, "Unexpected response type %T — skipping\n", apiResp)
		return "", false, nil, nil
	}

	// --- Step 7: ID template ---
	fmt.Fprintf(w, "Enter import ID template for %s:\n", resourceType)
	fmt.Fprintln(w, "(use .field for plan values, $var for vars, $<resolver_name> for this resolver)")
	fmt.Fprint(w, "> ")
	idTemplate, err := scanLine(scanner)
	if err != nil {
		return "", false, nil, err
	}
	if idTemplate == "" {
		fmt.Fprintln(w, "No ID template provided — skipping")
		return "", false, nil, nil
	}

	// Build the full use list for the type mapping: deps + this resolver.
	typeUse := make([]string, 0, len(deps)+1)
	typeUse = append(typeUse, deps...)
	typeUse = append(typeUse, resolverName)

	result := &ResolverResult{
		Name: resolverName,
		Get:  getPath,
		Pick: pick,
		Use:  deps,
		TypeMapping: config.TypeMapping{
			Use: typeUse,
			ID:  idTemplate,
		},
	}

	// --- Step 8: Preview and confirm ---
	fmt.Fprintln(w, "\nGenerated:")
	formatResolverYAML(w, result, resourceType)

	fmt.Fprint(w, "Save to config? [y/N]: ")
	confirmLine, err := scanLine(scanner)
	if err != nil {
		return "", false, nil, err
	}
	doSave := strings.EqualFold(confirmLine, "y") || strings.EqualFold(confirmLine, "yes")

	// Use the resolver name as a placeholder ID (real rendering happens via config).
	return "$" + resolverName, doSave, result, nil
}

// updateConfig reads the YAML config at configPath, applies the mutate function
// to the parsed config, marshals it, and writes it back.
func updateConfig(configPath string, mutate func(*config.Config)) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("prompt: read config %q: %w", configPath, err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("prompt: unmarshal config %q: %w", configPath, err)
	}

	mutate(&cfg)

	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("prompt: marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0o644); err != nil {
		return fmt.Errorf("prompt: write config %q: %w", configPath, err)
	}

	return nil
}

// SaveTypeMapping reads the YAML config at configPath, appends (or overwrites)
// a type mapping for resourceType with the given idTemplate, and writes the
// file back.
func SaveTypeMapping(configPath string, resourceType string, idTemplate string) error {
	return updateConfig(configPath, func(cfg *config.Config) {
		if cfg.Types == nil {
			cfg.Types = make(map[string]config.TypeMapping)
		}
		cfg.Types[resourceType] = config.TypeMapping{ID: idTemplate}
	})
}

// SaveResolverAndType reads the YAML config at configPath, adds the resolver
// and type mapping from result, and writes the file back.
func SaveResolverAndType(configPath string, resourceType string, result *ResolverResult) error {
	return updateConfig(configPath, func(cfg *config.Config) {
		if cfg.Resolvers == nil {
			cfg.Resolvers = make(map[string]config.Resolver)
		}
		cfg.Resolvers[result.Name] = config.Resolver{
			Use:  result.Use,
			Get:  result.Get,
			Pick: result.Pick,
		}

		if cfg.Types == nil {
			cfg.Types = make(map[string]config.TypeMapping)
		}
		cfg.Types[resourceType] = result.TypeMapping
	})
}

// --- helpers ---

// scanLine reads the next line from scanner, trims whitespace, and returns it.
// Returns ("", nil) on EOF.
func scanLine(scanner *bufio.Scanner) (string, error) {
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("prompt: read: %w", err)
		}
		return "", nil // EOF
	}
	return strings.TrimSpace(scanner.Text()), nil
}

// manualIDWithScanner implements the ManualID flow using an already-created
// scanner so the caller's buffered state is preserved.
func manualIDWithScanner(scanner *bufio.Scanner, w io.Writer, address, resourceType string, fields map[string]string) (id string, save bool, err error) {
	if len(fields) > 0 {
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf(".%s = %q", k, fields[k]))
		}
		fmt.Fprintf(w, "Available fields: %s\n", strings.Join(parts, ", "))
	}

	fmt.Fprintf(w, "Enter import ID for %s (or 'skip'): ", address)
	line, scanErr := scanLine(scanner)
	if scanErr != nil {
		return "", false, scanErr
	}
	if line == "" || strings.EqualFold(line, "skip") {
		return "", false, nil
	}
	id = line

	fmt.Fprintf(w, "Save as template for type %s? [y/N]: ", resourceType)
	answer, scanErr := scanLine(scanner)
	if scanErr != nil {
		return id, false, scanErr
	}
	save = strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")

	return id, save, nil
}

// renderSimple performs basic template substitution for use in the API test
// call. It replaces .field references with plan field values and $var with
// vars/resolver values. This avoids a full Go template engine for the preview.
func renderSimple(tmpl string, fields map[string]string, vars map[string]string) string {
	result := tmpl

	// Replace .field references (longest keys first to avoid partial matches).
	type kv struct{ k, v string }
	var fieldList []kv
	for k, v := range fields {
		fieldList = append(fieldList, kv{k, v})
	}
	sort.Slice(fieldList, func(i, j int) bool {
		return len(fieldList[i].k) > len(fieldList[j].k)
	})
	for _, f := range fieldList {
		result = strings.ReplaceAll(result, "."+f.k, f.v)
	}

	// Replace $var references.
	var varList []kv
	for k, v := range vars {
		varList = append(varList, kv{k, v})
	}
	sort.Slice(varList, func(i, j int) bool {
		return len(varList[i].k) > len(varList[j].k)
	})
	for _, v := range varList {
		result = strings.ReplaceAll(result, "$"+v.k, v.v)
	}

	return result
}

// displayJSONSummary writes a brief human-readable summary of an API response.
func displayJSONSummary(w io.Writer, response interface{}) {
	switch v := response.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintf(w, "Response: JSON object with %d fields: %s\n", len(keys), strings.Join(keys, ", "))
		// Show a few sample values.
		shown := 0
		for _, k := range keys {
			if shown >= 5 {
				break
			}
			val := v[k]
			switch val.(type) {
			case map[string]interface{}, []interface{}:
				fmt.Fprintf(w, "  .%s = (nested)\n", k)
			default:
				fmt.Fprintf(w, "  .%s = %v\n", k, val)
			}
			shown++
		}

	case []interface{}:
		fmt.Fprintf(w, "Response: JSON array with %d elements\n", len(v))
		if len(v) > 0 {
			b, err := json.Marshal(v[0])
			if err == nil {
				fmt.Fprintf(w, "  First element: %s\n", string(b))
			}
		}

	default:
		fmt.Fprintf(w, "Response: %v\n", response)
	}
}

// formatResolverYAML writes a YAML preview of the generated resolver and type
// mapping to w.
func formatResolverYAML(w io.Writer, result *ResolverResult, resourceType string) {
	fmt.Fprintln(w, "  resolvers:")
	fmt.Fprintf(w, "    %s:\n", result.Name)
	if len(result.Use) > 0 {
		fmt.Fprintf(w, "      use: [%s]\n", strings.Join(result.Use, ", "))
	}
	fmt.Fprintf(w, "      get: %q\n", result.Get)
	switch p := result.Pick.(type) {
	case string:
		fmt.Fprintf(w, "      pick: %q\n", p)
	case map[string]interface{}:
		b, err := yaml.Marshal(p)
		if err == nil {
			lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
			fmt.Fprintln(w, "      pick:")
			for _, line := range lines {
				fmt.Fprintf(w, "        %s\n", line)
			}
		}
	}
	fmt.Fprintln(w, "  types:")
	fmt.Fprintf(w, "    %s:\n", resourceType)
	if len(result.TypeMapping.Use) > 0 {
		fmt.Fprintf(w, "      use: [%s]\n", strings.Join(result.TypeMapping.Use, ", "))
	}
	fmt.Fprintf(w, "      id: %q\n", result.TypeMapping.ID)
}
