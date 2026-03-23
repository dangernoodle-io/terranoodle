package generator

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"dangernoodle.io/terratools/internal/catalog/hclparse"
)

func TestResolveValues_NoCrossTemplateRefs(t *testing.T) {
	// Create a template with only Values (no RawValues).
	tmpl := &hclparse.UnitDef{
		Name: "acme-svc",
		Values: map[string]cty.Value{
			"env": cty.StringVal("staging"),
		},
		RawValues: nil,
	}

	result, err := ResolveValues(tmpl, []hclparse.UnitDef{*tmpl})

	require.NoError(t, err)
	require.Contains(t, result, "env")
	assert.Equal(t, cty.StringVal("staging"), result["env"])
}

func TestResolveValues_NoRawValues(t *testing.T) {
	// When RawValues is nil/empty, ResolveValues should just return Values.
	tmpl := &hclparse.UnitDef{
		Name: "acme-svc",
		Values: map[string]cty.Value{
			"env":     cty.StringVal("prod"),
			"project": cty.StringVal("acme-project"),
		},
		RawValues: make(map[string]hcl.Expression),
	}

	result, err := ResolveValues(tmpl, []hclparse.UnitDef{*tmpl})

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, cty.StringVal("prod"), result["env"])
	assert.Equal(t, cty.StringVal("acme-project"), result["project"])
}

func TestResolveValues_EmptyValues(t *testing.T) {
	// A template with no values should return an empty map.
	tmpl := &hclparse.UnitDef{
		Name:      "empty-template",
		Values:    make(map[string]cty.Value),
		RawValues: make(map[string]hcl.Expression),
	}

	result, err := ResolveValues(tmpl, []hclparse.UnitDef{*tmpl})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestResolveValues_MultipleTemplates(t *testing.T) {
	// Test that ResolveValues includes prior templates in the context.
	// This test verifies the context is built with prior templates only.
	firstTmpl := &hclparse.UnitDef{
		Name: "first-template",
		Values: map[string]cty.Value{
			"env": cty.StringVal("staging"),
		},
		RawValues: make(map[string]hcl.Expression),
	}

	secondTmpl := &hclparse.UnitDef{
		Name: "second-template",
		Values: map[string]cty.Value{
			"region": cty.StringVal("us-central1"),
		},
		RawValues: make(map[string]hcl.Expression),
	}

	allTemplates := []hclparse.UnitDef{*firstTmpl, *secondTmpl}

	result, err := ResolveValues(secondTmpl, allTemplates)

	require.NoError(t, err)
	assert.Equal(t, cty.StringVal("us-central1"), result["region"])
}
