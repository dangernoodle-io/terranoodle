package generator

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// traversalRef holds a values.xxx traversal and its byte range within the source.
type traversalRef struct {
	traversal hcl.Traversal
	startByte int
	endByte   int
}

// ResolveTemplate reads a catalog template terragrunt.hcl file and replaces
// all `values.xxx` traversals with literal values from the provided values map.
// The values map is keyed by the service name — for the project template, use
// the top-level project values; for service templates, use the service's values sub-map.
// Returns the resolved bytes, any warnings about unresolvable traversals, and any error.
func ResolveTemplate(templatePath string, values cty.Value) ([]byte, []string, error) {
	src, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading template %s: %w", templatePath, err)
	}

	file, diags := hclsyntax.ParseConfig(src, templatePath, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, nil, fmt.Errorf("parsing template %s: %s", templatePath, diags.Error())
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected body type in %s", templatePath)
	}

	// Collect all values.xxx traversals with their byte ranges.
	refs := collectValuesTraversals(body)

	if len(refs) == 0 {
		return src, nil, nil
	}

	// Sort in reverse order by start byte so we can replace without shifting offsets.
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].startByte > refs[j].startByte
	})

	// Deduplicate refs at identical positions (Variables() may return duplicates).
	deduped := make([]traversalRef, 0, len(refs))
	seen := make(map[int]bool)
	for _, ref := range refs {
		if !seen[ref.startByte] {
			seen[ref.startByte] = true
			deduped = append(deduped, ref)
		}
	}

	result := make([]byte, len(src))
	copy(result, src)

	var warnings []string

	for _, ref := range deduped {
		val, err := lookupTraversal(values, ref.traversal)
		if err != nil {
			// Collect a warning and skip — the template may have optional references
			// that don't apply to this service.
			warnings = append(warnings, fmt.Sprintf("%s: unresolved %s", templatePath, traversalString(ref.traversal)))
			continue
		}

		literal := hclwrite.TokensForValue(val).Bytes()

		// Indent multi-line literals to match the column where the traversal starts.
		literal = indentLiteral(literal, ref.startByte, result)

		// Replace the traversal bytes with the literal.
		result = append(result[:ref.startByte], append(literal, result[ref.endByte:]...)...)
	}

	return result, warnings, nil
}

// traversalString formats an hcl.Traversal as a dotted path (e.g. "values.foo.bar").
func traversalString(t hcl.Traversal) string {
	var parts []string
	for _, step := range t {
		switch s := step.(type) {
		case hcl.TraverseRoot:
			parts = append(parts, s.Name)
		case hcl.TraverseAttr:
			parts = append(parts, s.Name)
		case hcl.TraverseIndex:
			parts = append(parts, fmt.Sprintf("[%s]", s.Key.AsString()))
		}
	}
	return strings.Join(parts, ".")
}

// collectValuesTraversals walks an hclsyntax body recursively and returns all
// traversals that start with the root "values", along with their byte ranges.
func collectValuesTraversals(body *hclsyntax.Body) []traversalRef {
	var refs []traversalRef

	for _, attr := range body.Attributes {
		refs = append(refs, findValuesRefs(attr.Expr)...)
	}

	for _, block := range body.Blocks {
		refs = append(refs, collectValuesTraversals(block.Body)...)
	}

	return refs
}

// findValuesRefs finds all traversals starting with "values" in an expression.
func findValuesRefs(expr hclsyntax.Expression) []traversalRef {
	var refs []traversalRef

	for _, traversal := range expr.Variables() {
		if len(traversal) == 0 {
			continue
		}
		root, ok := traversal[0].(hcl.TraverseRoot)
		if !ok || root.Name != "values" {
			continue
		}

		srcRange := traversal.SourceRange()
		refs = append(refs, traversalRef{
			traversal: traversal,
			startByte: srcRange.Start.Byte,
			endByte:   srcRange.End.Byte,
		})
	}

	return refs
}

// lookupTraversal follows the traversal steps (skipping the root "values") into
// the provided cty.Value and returns the resolved value.
func lookupTraversal(root cty.Value, traversal hcl.Traversal) (cty.Value, error) {
	if len(traversal) < 2 {
		// Just "values" with no attribute — return the whole object.
		return root, nil
	}

	current := root
	// Skip traversal[0] (the "values" root) and walk the rest.
	for _, step := range traversal[1:] {
		switch s := step.(type) {
		case hcl.TraverseAttr:
			if !current.Type().IsObjectType() {
				return cty.NilVal, fmt.Errorf("cannot traverse attribute %q on non-object type %s", s.Name, current.Type().FriendlyName())
			}
			attrType := current.Type()
			if !attrType.HasAttribute(s.Name) {
				return cty.NilVal, fmt.Errorf("object has no attribute %q", s.Name)
			}
			current = current.GetAttr(s.Name)

		case hcl.TraverseIndex:
			if !current.Type().IsMapType() && !current.Type().IsObjectType() {
				return cty.NilVal, fmt.Errorf("cannot index non-map/object type %s", current.Type().FriendlyName())
			}
			idx := s.Key
			if current.Type().IsObjectType() {
				if idx.Type() != cty.String {
					return cty.NilVal, fmt.Errorf("object index must be string")
				}
				attrName := idx.AsString()
				if !current.Type().HasAttribute(attrName) {
					return cty.NilVal, fmt.Errorf("object has no attribute %q", attrName)
				}
				current = current.GetAttr(attrName)
			} else {
				val := current.Index(idx)
				if !val.IsKnown() {
					return cty.NilVal, fmt.Errorf("index not found in map")
				}
				current = val
			}

		default:
			return cty.NilVal, fmt.Errorf("unsupported traversal step type %T", step)
		}
	}

	return current, nil
}

// indentLiteral indents all lines of a multi-line literal (after the first line)
// to match the leading whitespace of the line where the value is being inserted.
// This ensures nested objects/maps stay properly aligned with the surrounding HCL.
func indentLiteral(literal []byte, insertPos int, content []byte) []byte {
	if !bytes.Contains(literal, []byte("\n")) {
		return literal
	}

	// Find the start of the current line by scanning back to the previous newline.
	lineStart := 0
	for i := insertPos - 1; i >= 0; i-- {
		if content[i] == '\n' {
			lineStart = i + 1
			break
		}
	}

	// Extract the leading whitespace of the current line.
	var indent []byte
	for i := lineStart; i < insertPos; i++ {
		if content[i] == ' ' || content[i] == '\t' {
			indent = append(indent, content[i])
		} else {
			break
		}
	}

	if len(indent) == 0 {
		return literal
	}

	lines := bytes.Split(literal, []byte("\n"))

	var result []byte
	for i, line := range lines {
		if i > 0 {
			result = append(result, '\n')
			if len(line) > 0 {
				result = append(result, indent...)
			}
		}
		result = append(result, line...)
	}

	return result
}
