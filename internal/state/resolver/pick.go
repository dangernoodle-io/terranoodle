package resolver

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/itchyny/gojq"

	"dangernoodle.io/terra-tools/internal/state/config"
)

// Pick extracts a string value from an API response using the pick expression
// defined in the config. raw is the raw pick value (string or map) as parsed
// from YAML; response is the decoded JSON body returned by APIClient.Get.
//
//   - Simple string pick (e.g. "id"): response must be a JSON object; returns
//     response[field] as a string.
//   - jq expression (e.g. ".[] | select(.name == "main") | .id"): evaluated
//     against response using gojq; returns the first non-error result as a string.
//   - PickExpr (where + field): response must be a JSON array of objects; finds
//     the first element where all where key/value pairs match, then returns
//     element[field] as a string.
func Pick(response interface{}, raw interface{}, ctx map[string]interface{}) (string, error) {
	field, expr, jqExpr, err := config.ParsePick(raw)
	if err != nil {
		return "", fmt.Errorf("pick: %w", err)
	}

	if jqExpr != "" {
		return pickJQ(response, jqExpr, ctx)
	}

	if expr == nil {
		// Simple string case: response must be a map.
		m, ok := response.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("pick: expected JSON object for field %q, got %T", field, response)
		}
		v, ok := m[field]
		if !ok {
			return "", fmt.Errorf("pick: field %q not found in response", field)
		}
		return toString(v), nil
	}

	// PickExpr case: response must be a slice of maps.
	slice, ok := response.([]interface{})
	if !ok {
		return "", fmt.Errorf("pick: expected JSON array for where/field pick, got %T", response)
	}

	for _, elem := range slice {
		m, ok := elem.(map[string]interface{})
		if !ok {
			continue
		}
		if matchesWhere(m, expr.Where, ctx) {
			v, ok := m[expr.Field]
			if !ok {
				return "", fmt.Errorf("pick: field %q not found in matching element", expr.Field)
			}
			return toString(v), nil
		}
	}

	return "", fmt.Errorf("pick: no element matched where conditions %v", expr.Where)
}

// pickJQ evaluates a jq expression against response and returns the first
// non-error result as a string. The expression may contain Go template
// references (e.g. {{ .name }}) which are rendered using ctx before the jq
// expression is parsed and executed.
func pickJQ(response interface{}, jqExpr string, ctx map[string]interface{}) (string, error) {
	// Render any Go template references embedded in the jq expression.
	rendered, err := renderWhereValue(jqExpr, ctx)
	if err != nil {
		return "", fmt.Errorf("pick: jq expression template render: %w", err)
	}

	query, err := gojq.Parse(rendered)
	if err != nil {
		return "", fmt.Errorf("pick: jq parse %q: %w", rendered, err)
	}

	code, err := gojq.Compile(query)
	if err != nil {
		return "", fmt.Errorf("pick: jq compile %q: %w", rendered, err)
	}

	iter := code.Run(response)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if _, isErr := v.(error); isErr {
			continue
		}
		return toString(v), nil
	}

	return "", fmt.Errorf("pick: jq expression %q yielded no results", rendered)
}

// matchesWhere reports whether all key/value pairs in where match the
// corresponding values in m. The where values are treated as Go templates
// and rendered against ctx before comparison.
func matchesWhere(m map[string]interface{}, where map[string]string, ctx map[string]interface{}) bool {
	for k, pattern := range where {
		v, ok := m[k]
		if !ok {
			return false
		}
		want, err := renderWhereValue(pattern, ctx)
		if err != nil {
			return false
		}
		if toString(v) != want {
			return false
		}
	}
	return true
}

// renderWhereValue renders a where clause value as a Go template. If parsing
// fails (e.g., it's a plain string), it returns the value as-is.
func renderWhereValue(pattern string, ctx map[string]interface{}) (string, error) {
	if !strings.Contains(pattern, "{{") {
		return pattern, nil
	}
	tmpl, err := template.New("where").Funcs(templateFuncs).Parse(pattern)
	if err != nil {
		return "", fmt.Errorf("pick: parse where template %q: %w", pattern, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("pick: execute where template %q: %w", pattern, err)
	}
	return buf.String(), nil
}

// toString converts an interface{} value to its string representation.
// Numbers are formatted with %v (which avoids scientific notation for integers
// decoded from JSON as float64 when they are whole numbers).
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		// If it's a whole number, format without a decimal point.
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	default:
		return fmt.Sprintf("%v", v)
	}
}
