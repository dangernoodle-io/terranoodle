package resolver

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"text/template"

	tfjson "github.com/hashicorp/terraform-json"
)

// templateFuncs returns the FuncMap used by all ID templates.
var templateFuncs = template.FuncMap{
	// pathEncode joins args with "/" and URL-encodes each segment.
	"pathEncode": func(parts ...string) string {
		encoded := make([]string, len(parts))
		for i, p := range parts {
			encoded[i] = url.PathEscape(p)
		}
		return strings.Join(encoded, "/")
	},

	// splitIndex splits s by sep and returns the element at index i.
	// Returns an empty string if the index is out of range.
	"splitIndex": func(s, sep string, i int) string {
		parts := strings.Split(s, sep)
		if i < 0 || i >= len(parts) {
			return ""
		}
		return parts[i]
	},

	// urlencode URL-encodes a single string.
	"urlencode": func(s string) string {
		return url.QueryEscape(s)
	},
}

// preprocessTemplate rewrites $varname references inside Go template
// action delimiters to .varname for all names that are known var or
// resolver keys.  This lets callers write {{ $project }} in their YAML
// templates and have those references resolve against the flat data map
// rather than Go's local template variables.
//
// Only whole-word occurrences (i.e. $name not immediately followed by
// another identifier character) are rewritten, and only for names that
// are explicitly provided — unknown $names are left untouched so that
// genuine Go template variable assignments still work.
func preprocessTemplate(tmpl string, varNames, resolverNames []string) string {
	known := make(map[string]struct{}, len(varNames)+len(resolverNames))
	for _, n := range varNames {
		known[n] = struct{}{}
	}
	for _, n := range resolverNames {
		known[n] = struct{}{}
	}

	if len(known) == 0 {
		return tmpl
	}

	// Walk the string looking for {{ … }} action blocks and rewrite $name
	// inside them.  We do a simple byte scan rather than pulling in a full
	// parser because the templates are short and well-structured.
	var out strings.Builder
	out.Grow(len(tmpl))

	rest := tmpl
	for {
		// Find the next opening delimiter.
		open := strings.Index(rest, "{{")
		if open == -1 {
			out.WriteString(rest)
			break
		}

		// Copy everything before the action verbatim.
		out.WriteString(rest[:open])
		rest = rest[open:]

		// Find the matching close delimiter.
		closeIdx := strings.Index(rest, "}}")
		if closeIdx == -1 {
			// Malformed template — copy the rest and bail.
			out.WriteString(rest)
			break
		}
		closeIdx += 2 // include "}}"

		action := rest[:closeIdx]
		rest = rest[closeIdx:]

		// Rewrite known $name references inside this action block.
		out.WriteString(rewriteDollarRefs(action, known))
	}

	return out.String()
}

// rewriteDollarRefs replaces $name with .name for each name in known
// within a single template action string (including the delimiters).
func rewriteDollarRefs(action string, known map[string]struct{}) string {
	var out strings.Builder
	out.Grow(len(action))

	i := 0
	for i < len(action) {
		if action[i] != '$' {
			out.WriteByte(action[i])
			i++
			continue
		}

		// Collect the identifier that follows '$'.
		j := i + 1
		for j < len(action) && isIdentChar(action[j]) {
			j++
		}

		name := action[i+1 : j]
		if _, ok := known[name]; ok {
			out.WriteByte('.')
			out.WriteString(name)
		} else {
			// Unknown — preserve the original $name.
			out.WriteString(action[i:j])
		}
		i = j
	}

	return out.String()
}

// isIdentChar reports whether b can appear in a Go/template identifier
// after the first character.
func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') ||
		b == '_'
}

// RenderID executes idTemplate against the data derived from rc, vars,
// and resolverResults, returning the rendered import ID string.
//
// Template data map (flat, later entries win):
//  1. All Change.After values from the plan (e.g. .name, .zone, .project).
//  2. ".key"     — rc.Index (for_each key or count index, stringified).
//  3. ".address" — rc.Address.
//  4. Config vars (keyed without the "$" prefix).
//  5. Resolver results (keyed without the "$" prefix).
//
// In the YAML template, vars and resolver results are referenced with a
// "$" prefix (e.g. {{ $project_id }}). preprocessTemplate converts those
// to dot-notation before parsing so they resolve against the flat map.
// BuildContext builds the flat template data map for a resource change.
func BuildContext(
	rc *tfjson.ResourceChange,
	vars map[string]string,
	resolverResults map[string]string,
) map[string]interface{} {
	data := make(map[string]interface{})

	// 1. Plan values.
	if rc.Change != nil && rc.Change.After != nil {
		if afterMap, ok := rc.Change.After.(map[string]interface{}); ok {
			for k, v := range afterMap {
				data[k] = v
			}
		}
	}

	// 2. Special plan fields.
	if rc.Index != nil {
		data["key"] = fmt.Sprintf("%v", rc.Index)
	} else {
		data["key"] = ""
	}
	data["address"] = rc.Address

	// 3. Config vars.
	for k, v := range vars {
		data[k] = v
	}

	// 4. Resolver results (override everything).
	for k, v := range resolverResults {
		data[k] = v
	}

	return data
}

func RenderID(
	idTemplate string,
	rc *tfjson.ResourceChange,
	vars map[string]string,
	resolverResults map[string]string,
) (string, error) {
	data := BuildContext(rc, vars, resolverResults)

	varNames := make([]string, 0, len(vars))
	for k := range vars {
		varNames = append(varNames, k)
	}
	resolverNames := make([]string, 0, len(resolverResults))
	for k := range resolverResults {
		resolverNames = append(resolverNames, k)
	}

	processed := preprocessTemplate(idTemplate, varNames, resolverNames)

	t, err := template.New("id").Funcs(templateFuncs).Parse(processed)
	if err != nil {
		return "", fmt.Errorf("resolver: parse template for %s: %w", rc.Address, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("resolver: render template for %s: %w", rc.Address, err)
	}

	return buf.String(), nil
}
