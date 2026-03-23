package generator

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"

	"dangernoodle.io/terratools/internal/catalog/hclparse"
)

// ResolveValues takes a template's raw values and resolves all template.xxx cross-references
// using values from other templates in the definition.
//
// It builds an hcl.EvalContext where `template` is a cty map variable keyed by
// template name (original, with hyphens). Users reference other templates via index
// syntax: template["acme-corp-main"].database.name
func ResolveValues(tmpl *hclparse.UnitDef, allTemplates []hclparse.UnitDef) (map[string]cty.Value, error) {
	// Build map of templates that appear before this one, keyed by original name.
	templateVars := make(map[string]cty.Value)

	for _, s := range allTemplates {
		if s.Name == tmpl.Name {
			break
		}
		if len(s.Values) > 0 {
			templateVars[s.Name] = cty.ObjectVal(s.Values)
		} else {
			templateVars[s.Name] = cty.EmptyObjectVal
		}
	}

	// Build eval context with `template` as a map variable (supports index syntax).
	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{},
	}
	if len(templateVars) > 0 {
		// Use ObjectVal rather than MapVal because each template's values object
		// may have a different type structure (different services enabled).
		ctx.Variables["template"] = cty.ObjectVal(templateVars)
	}

	// Start with already-evaluated values.
	result := make(map[string]cty.Value, len(tmpl.Values))
	for k, v := range tmpl.Values {
		result[k] = v
	}

	// Resolve entries that were NOT already evaluated (contain template.xxx refs).
	for key, rawExpr := range tmpl.RawValues {
		if _, alreadyEvaluated := result[key]; alreadyEvaluated {
			continue
		}

		// Validate all template["..."] references point to known prior templates.
		for _, traversal := range rawExpr.Variables() {
			if len(traversal) < 2 {
				continue
			}
			root, ok := traversal[0].(hcl.TraverseRoot)
			if !ok || root.Name != "template" {
				continue
			}

			// Index syntax: template["acme-corp-main"] produces TraverseIndex.
			var refKey string
			switch step := traversal[1].(type) {
			case hcl.TraverseIndex:
				if step.Key.Type() == cty.String {
					refKey = step.Key.AsString()
				}
			case hcl.TraverseAttr:
				refKey = step.Name
			default:
				continue
			}
			if refKey == "" {
				continue
			}

			if _, exists := templateVars[refKey]; !exists {
				existsLater := false
				for _, s := range allTemplates {
					if s.Name == refKey {
						existsLater = true
						break
					}
				}
				if existsLater {
					return nil, fmt.Errorf(
						"template %q: values.%s references template %q which has not been evaluated yet "+
							"(dependent templates must appear after their dependencies in the definition)",
						tmpl.Name, key, refKey)
				}
				return nil, fmt.Errorf(
					"template %q: values.%s references unknown template %q",
					tmpl.Name, key, refKey)
			}
		}

		val, diags := rawExpr.Value(ctx)
		if diags.HasErrors() {
			return nil, fmt.Errorf("template %q: evaluating values.%s: %s", tmpl.Name, key, diags.Error())
		}
		result[key] = val
	}

	return result, nil
}
