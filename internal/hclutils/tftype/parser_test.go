package tftype

import (
	"fmt"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestParseConstraint_NilExpression(t *testing.T) {
	result, err := ParseConstraint(nil)

	assert.Nil(t, err)
	assert.Equal(t, cty.DynamicPseudoType, result)
}

func TestParseConstraint_StringType(t *testing.T) {
	expr := parseTypeExpr(t, "string")

	result, err := ParseConstraint(expr)

	require.Nil(t, err)
	assert.Equal(t, cty.String, result)
}

func TestParseConstraint_NumberType(t *testing.T) {
	expr := parseTypeExpr(t, "number")

	result, err := ParseConstraint(expr)

	require.Nil(t, err)
	assert.Equal(t, cty.Number, result)
}

func TestParseConstraint_BoolType(t *testing.T) {
	expr := parseTypeExpr(t, "bool")

	result, err := ParseConstraint(expr)

	require.Nil(t, err)
	assert.Equal(t, cty.Bool, result)
}

func TestParseConstraint_ListString(t *testing.T) {
	expr := parseTypeExpr(t, "list(string)")

	result, err := ParseConstraint(expr)

	require.Nil(t, err)
	assert.Equal(t, cty.List(cty.String), result)
}

func TestParseConstraint_SetNumber(t *testing.T) {
	expr := parseTypeExpr(t, "set(number)")

	result, err := ParseConstraint(expr)

	require.Nil(t, err)
	assert.Equal(t, cty.Set(cty.Number), result)
}

func TestParseConstraint_MapString(t *testing.T) {
	expr := parseTypeExpr(t, "map(string)")

	result, err := ParseConstraint(expr)

	require.Nil(t, err)
	assert.Equal(t, cty.Map(cty.String), result)
}

func TestParseConstraint_ObjectWithAttrs(t *testing.T) {
	expr := parseTypeExpr(t, "object({ name = string, count = number })")

	result, err := ParseConstraint(expr)

	require.Nil(t, err)
	expected := cty.Object(map[string]cty.Type{
		"name":  cty.String,
		"count": cty.Number,
	})
	assert.Equal(t, expected, result)
}

func TestParseConstraint_ObjectEmpty(t *testing.T) {
	expr := parseTypeExpr(t, "object({})")

	result, err := ParseConstraint(expr)

	require.Nil(t, err)
	expected := cty.Object(map[string]cty.Type{})
	assert.Equal(t, expected, result)
}

func TestParseConstraint_TupleTypes(t *testing.T) {
	expr := parseTypeExpr(t, "tuple([string, number, bool])")

	result, err := ParseConstraint(expr)

	require.Nil(t, err)
	expected := cty.Tuple([]cty.Type{cty.String, cty.Number, cty.Bool})
	assert.Equal(t, expected, result)
}

func TestParseConstraint_InvalidSyntax(t *testing.T) {
	expr := parseTypeExpr(t, "invalid-type-name")

	_, err := ParseConstraint(expr)

	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "parsing type constraint")
}

func TestParseConstraint_NestedObject(t *testing.T) {
	expr := parseTypeExpr(t, "object({ config = object({ region = string }) })")

	result, err := ParseConstraint(expr)

	require.Nil(t, err)
	expected := cty.Object(map[string]cty.Type{
		"config": cty.Object(map[string]cty.Type{
			"region": cty.String,
		}),
	})
	assert.Equal(t, expected, result)
}

func TestParseConstraint_ListOfObjects(t *testing.T) {
	expr := parseTypeExpr(t, "list(object({ id = string, name = string }))")

	result, err := ParseConstraint(expr)

	require.Nil(t, err)
	expected := cty.List(cty.Object(map[string]cty.Type{
		"id":   cty.String,
		"name": cty.String,
	}))
	assert.Equal(t, expected, result)
}

// parseTypeExpr is a helper that extracts a type expression from a variable block.
func parseTypeExpr(t *testing.T, typeStr string) hcl.Expression {
	t.Helper()

	src := fmt.Sprintf(`variable "test" { type = %s }`, typeStr)
	file, diags := hclsyntax.ParseConfig([]byte(src), "test.tf", hcl.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors(), diags.Error())

	content, _, diags := file.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "variable", LabelNames: []string{"name"}},
		},
	})
	require.False(t, diags.HasErrors(), diags.Error())
	require.NotEmpty(t, content.Blocks)

	attrs, diags := content.Blocks[0].Body.JustAttributes()
	require.False(t, diags.HasErrors(), diags.Error())

	return attrs["type"].Expr
}
