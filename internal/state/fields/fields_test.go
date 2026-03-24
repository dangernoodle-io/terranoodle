package fields

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractStrings_StringValues(t *testing.T) {
	after := map[string]interface{}{
		"name":   "example-bucket",
		"region": "us-west-2",
	}
	result := ExtractStrings(after)
	assert.Equal(t, map[string]string{
		"name":   "example-bucket",
		"region": "us-west-2",
	}, result)
}

func TestExtractStrings_BoolValues(t *testing.T) {
	after := map[string]interface{}{
		"enabled": true,
		"public":  false,
	}
	result := ExtractStrings(after)
	assert.Equal(t, map[string]string{
		"enabled": "true",
		"public":  "false",
	}, result)
}

func TestExtractStrings_FloatWholeNumbers(t *testing.T) {
	after := map[string]interface{}{
		"count": float64(3),
		"size":  float64(100),
	}
	result := ExtractStrings(after)
	assert.Equal(t, map[string]string{
		"count": "3",
		"size":  "100",
	}, result)
}

func TestExtractStrings_FloatDecimalNumbers(t *testing.T) {
	after := map[string]interface{}{
		"ratio": float64(1.5),
		"rate":  float64(0.25),
	}
	result := ExtractStrings(after)
	assert.Equal(t, map[string]string{
		"ratio": "1.5",
		"rate":  "0.25",
	}, result)
}

func TestExtractStrings_MixedTypes(t *testing.T) {
	after := map[string]interface{}{
		"name":      "example-app",
		"enabled":   true,
		"count":     float64(5),
		"threshold": float64(2.5),
	}
	result := ExtractStrings(after)
	assert.Equal(t, map[string]string{
		"name":      "example-app",
		"enabled":   "true",
		"count":     "5",
		"threshold": "2.5",
	}, result)
}

func TestExtractStrings_NilInput(t *testing.T) {
	result := ExtractStrings(nil)
	assert.Equal(t, map[string]string{}, result)
}

func TestExtractStrings_NonMapType(t *testing.T) {
	result := ExtractStrings("not a map")
	assert.Equal(t, map[string]string{}, result)
}

func TestExtractStrings_NestedTypesSkipped(t *testing.T) {
	after := map[string]interface{}{
		"name": "test-resource",
		"tags": map[string]interface{}{
			"env": "staging",
		},
		"config": []interface{}{"a", "b"},
	}
	result := ExtractStrings(after)
	// nested map and list should be skipped
	assert.Equal(t, map[string]string{"name": "test-resource"}, result)
	assert.NotContains(t, result, "tags")
	assert.NotContains(t, result, "config")
}

func TestExtractStrings_EmptyMap(t *testing.T) {
	after := map[string]interface{}{}
	result := ExtractStrings(after)
	assert.Equal(t, map[string]string{}, result)
}

func TestExtractStrings_OnlySkippedTypes(t *testing.T) {
	after := map[string]interface{}{
		"nested": map[string]interface{}{"key": "value"},
		"list":   []interface{}{1, 2, 3},
	}
	result := ExtractStrings(after)
	assert.Equal(t, map[string]string{}, result)
}
