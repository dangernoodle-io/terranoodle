package resolver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// pickJQ tests
// ---------------------------------------------------------------------------

func TestPickJQ_SimpleFieldAccess(t *testing.T) {
	response := map[string]interface{}{"id": "123", "name": "foo"}
	result, err := pickJQ(response, ".id", nil)
	require.NoError(t, err)
	assert.Equal(t, "123", result)
}

func TestPickJQ_ArrayFilter(t *testing.T) {
	response := []interface{}{
		map[string]interface{}{"name": "main", "id": "1"},
		map[string]interface{}{"name": "dev", "id": "2"},
	}
	result, err := pickJQ(response, `.[] | select(.name == "main") | .id`, nil)
	require.NoError(t, err)
	assert.Equal(t, "1", result)
}

func TestPickJQ_NestedAccess(t *testing.T) {
	response := map[string]interface{}{
		"data": map[string]interface{}{
			"nested": map[string]interface{}{
				"value": "found",
			},
		},
	}
	result, err := pickJQ(response, ".data.nested.value", nil)
	require.NoError(t, err)
	assert.Equal(t, "found", result)
}

func TestPickJQ_ArrayIndex(t *testing.T) {
	response := []interface{}{
		map[string]interface{}{"id": "first"},
		map[string]interface{}{"id": "second"},
	}
	result, err := pickJQ(response, ".[0].id", nil)
	require.NoError(t, err)
	assert.Equal(t, "first", result)
}

func TestPickJQ_ComplexSelect(t *testing.T) {
	response := map[string]interface{}{
		"network_interfaces": []interface{}{
			map[string]interface{}{"primary": false, "ip_address": "10.0.0.1"},
			map[string]interface{}{"primary": true, "ip_address": "10.0.0.2"},
		},
	}
	result, err := pickJQ(response, ".network_interfaces[] | select(.primary == true) | .ip_address", nil)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.2", result)
}

func TestPickJQ_NoMatch(t *testing.T) {
	response := []interface{}{
		map[string]interface{}{"name": "other", "id": "99"},
	}
	_, err := pickJQ(response, `.[] | select(.name == "missing") | .id`, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yielded no results")
}

func TestPickJQ_InvalidSyntax(t *testing.T) {
	response := map[string]interface{}{"id": "1"}
	_, err := pickJQ(response, ".id ||| bad syntax !!!", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jq parse")
}

func TestPickJQ_TemplateInExpression(t *testing.T) {
	// jq expression containing a Go template ref resolved via ctx.
	response := []interface{}{
		map[string]interface{}{"name": "alpha", "id": "A"},
		map[string]interface{}{"name": "beta", "id": "B"},
	}
	ctx := map[string]interface{}{"target": "beta"}
	result, err := pickJQ(response, `.[] | select(.name == "{{ .target }}") | .id`, ctx)
	require.NoError(t, err)
	assert.Equal(t, "B", result)
}

// ---------------------------------------------------------------------------
// Pick (top-level) jq dispatch tests
// ---------------------------------------------------------------------------

func TestPick_DispatchesToJQ(t *testing.T) {
	// A raw pick value that is a jq expression should be dispatched to pickJQ.
	response := []interface{}{
		map[string]interface{}{"env": "prod", "id": "p-1"},
		map[string]interface{}{"env": "dev", "id": "d-1"},
	}
	result, err := Pick(response, `.[] | select(.env == "prod") | .id`, nil)
	require.NoError(t, err)
	assert.Equal(t, "p-1", result)
}

// ---------------------------------------------------------------------------
// ParsePick jq detection tests (via the config package, tested here for
// integration with the Pick function)
// ---------------------------------------------------------------------------

func TestPick_SimpleField(t *testing.T) {
	response := map[string]interface{}{"id": "xyz"}
	result, err := Pick(response, "id", nil)
	require.NoError(t, err)
	assert.Equal(t, "xyz", result)
}

func TestPick_JQExpressionWithArrayIterator(t *testing.T) {
	// ".items[].name" — contains "[]" so treated as jq.
	response := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"name": "first"},
		},
	}
	result, err := Pick(response, ".items[].name", nil)
	require.NoError(t, err)
	assert.Equal(t, "first", result)
}
