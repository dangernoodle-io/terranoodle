package tftype

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestIsAssignable(t *testing.T) {
	tests := []struct {
		name           string
		valueType      cty.Type
		constraintType cty.Type
		expected       bool
	}{
		// Basic type matching
		{
			name:           "string to string",
			valueType:      cty.String,
			constraintType: cty.String,
			expected:       true,
		},
		{
			name:           "number to string (convertible)",
			valueType:      cty.Number,
			constraintType: cty.String,
			expected:       true,
		},
		{
			name:           "number to bool",
			valueType:      cty.Number,
			constraintType: cty.Bool,
			expected:       false,
		},

		// DynamicPseudoType handling
		{
			name:           "DynamicPseudoType to string",
			valueType:      cty.DynamicPseudoType,
			constraintType: cty.String,
			expected:       true,
		},
		{
			name:           "string to DynamicPseudoType",
			valueType:      cty.String,
			constraintType: cty.DynamicPseudoType,
			expected:       true,
		},
		{
			name:           "number to DynamicPseudoType",
			valueType:      cty.Number,
			constraintType: cty.DynamicPseudoType,
			expected:       true,
		},

		// Object types
		{
			name: "object with matching attrs",
			valueType: cty.Object(map[string]cty.Type{
				"a": cty.String,
				"b": cty.Number,
			}),
			constraintType: cty.Object(map[string]cty.Type{
				"a": cty.String,
				"b": cty.Number,
			}),
			expected: true,
		},
		{
			name: "object with extra attr",
			valueType: cty.Object(map[string]cty.Type{
				"a": cty.String,
				"b": cty.String,
			}),
			constraintType: cty.Object(map[string]cty.Type{
				"a": cty.String,
			}),
			expected: false,
		},

		// List types
		{
			name:           "list(string) to list(string)",
			valueType:      cty.List(cty.String),
			constraintType: cty.List(cty.String),
			expected:       true,
		},
		{
			name:           "list(number) to list(string) (convertible)",
			valueType:      cty.List(cty.Number),
			constraintType: cty.List(cty.String),
			expected:       true,
		},

		// Set types
		{
			name:           "set(string) to set(string)",
			valueType:      cty.Set(cty.String),
			constraintType: cty.Set(cty.String),
			expected:       true,
		},

		// Map types
		{
			name:           "map(string) to map(string)",
			valueType:      cty.Map(cty.String),
			constraintType: cty.Map(cty.String),
			expected:       true,
		},

		// Tuple to list
		{
			name:           "tuple(string, string) to list(string)",
			valueType:      cty.Tuple([]cty.Type{cty.String, cty.String}),
			constraintType: cty.List(cty.String),
			expected:       true,
		},
		{
			name:           "tuple(string, number) to list(string) (convertible)",
			valueType:      cty.Tuple([]cty.Type{cty.String, cty.Number}),
			constraintType: cty.List(cty.String),
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAssignable(tt.valueType, tt.constraintType)
			assert.Equal(t, tt.expected, result, "IsAssignable(%v, %v)", tt.valueType, tt.constraintType)
		})
	}
}

func TestExtraAttrs(t *testing.T) {
	tests := []struct {
		name           string
		valueType      cty.Type
		constraintType cty.Type
		expectErrors   bool
		errorContains  []string
	}{
		// Compatible types
		{
			name:           "object matching constraint",
			valueType:      cty.Object(map[string]cty.Type{"a": cty.String}),
			constraintType: cty.Object(map[string]cty.Type{"a": cty.String}),
			expectErrors:   false,
		},
		{
			name:           "simple types match",
			valueType:      cty.String,
			constraintType: cty.String,
			expectErrors:   false,
		},

		// Object with extra attributes
		{
			name: "object with extra attr",
			valueType: cty.Object(map[string]cty.Type{
				"a": cty.String,
				"b": cty.String,
			}),
			constraintType: cty.Object(map[string]cty.Type{
				"a": cty.String,
			}),
			expectErrors:  true,
			errorContains: []string{"unexpected attribute", "b"},
		},

		// Nested object with extra attribute
		{
			name: "nested object with extra attr",
			valueType: cty.Object(map[string]cty.Type{
				"config": cty.Object(map[string]cty.Type{
					"a": cty.String,
					"b": cty.String,
				}),
			}),
			constraintType: cty.Object(map[string]cty.Type{
				"config": cty.Object(map[string]cty.Type{
					"a": cty.String,
				}),
			}),
			expectErrors:  true,
			errorContains: []string{"config.b"},
		},

		// Type mismatch (not about attributes)
		{
			name:           "type mismatch string vs number",
			valueType:      cty.String,
			constraintType: cty.Number,
			expectErrors:   true,
			errorContains:  []string{"type"},
		},

		// Map with object constraint
		{
			name: "map(object) constraint with object value",
			valueType: cty.Object(map[string]cty.Type{
				"key1": cty.Object(map[string]cty.Type{
					"name":  cty.String,
					"extra": cty.Bool,
				}),
			}),
			constraintType: cty.Map(cty.Object(map[string]cty.Type{
				"name": cty.String,
			})),
			expectErrors:  true,
			errorContains: []string{"key1.extra"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ExtraAttrs(tt.valueType, tt.constraintType)

			if tt.expectErrors {
				require.NotEmpty(t, errs, "expected errors but got none")
				var allErrors string
				for _, err := range errs {
					allErrors += err + "; "
				}
				for _, contains := range tt.errorContains {
					assert.Contains(t, allErrors, contains, "error message should contain %q", contains)
				}
			} else {
				assert.Empty(t, errs, "expected no errors but got: %v", errs)
			}
		})
	}
}

func TestCheckAssignableListOfObjects(t *testing.T) {
	// list(object) constraint with list(object) value
	valueType := cty.List(cty.Object(map[string]cty.Type{
		"id":   cty.String,
		"name": cty.String,
	}))
	constraintType := cty.List(cty.Object(map[string]cty.Type{
		"id": cty.String,
	}))

	errs := ExtraAttrs(valueType, constraintType)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0], "name")
}

func TestCheckAssignableNestedList(t *testing.T) {
	// Nested list(list(string))
	valueType := cty.List(cty.List(cty.String))
	constraintType := cty.List(cty.List(cty.String))

	result := IsAssignable(valueType, constraintType)
	assert.True(t, result)
}

func TestCheckAssignableSetOfObjects(t *testing.T) {
	// set(object) constraint with set(object) value
	valueType := cty.Set(cty.Object(map[string]cty.Type{
		"id":   cty.String,
		"name": cty.String,
	}))
	constraintType := cty.Set(cty.Object(map[string]cty.Type{
		"id": cty.String,
	}))

	errs := ExtraAttrs(valueType, constraintType)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0], "name")
}
