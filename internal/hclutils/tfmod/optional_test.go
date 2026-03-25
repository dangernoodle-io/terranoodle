package tfmod

import (
	"fmt"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/require"
)

func parseTypeExpr(t *testing.T, typeExpr string) hcl.Expression {
	t.Helper()
	src := fmt.Sprintf(`variable "test" { type = %s }`, typeExpr)
	file, diags := hclsyntax.ParseConfig([]byte(src), "test.tf", hcl.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors(), diags.Error())
	content, diags := file.Body.Content(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "variable", LabelNames: []string{"name"}}},
	})
	require.False(t, diags.HasErrors())
	varContent, diags := content.Blocks[0].Body.Content(&hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{{Name: "type"}},
	})
	require.False(t, diags.HasErrors())
	return varContent.Attributes["type"].Expr
}

func TestHasOptionalWithoutDefault_Nil(t *testing.T) {
	result := HasOptionalWithoutDefault(nil)
	require.False(t, result)
}

func TestHasOptionalWithoutDefault_SimpleType(t *testing.T) {
	expr := parseTypeExpr(t, "string")
	result := HasOptionalWithoutDefault(expr)
	require.False(t, result)
}

func TestHasOptionalWithoutDefault_WithDefault(t *testing.T) {
	expr := parseTypeExpr(t, `object({ name = optional(string, "default") })`)
	result := HasOptionalWithoutDefault(expr)
	require.False(t, result)
}

func TestHasOptionalWithoutDefault_WithoutDefault(t *testing.T) {
	expr := parseTypeExpr(t, `object({ name = optional(string) })`)
	result := HasOptionalWithoutDefault(expr)
	require.True(t, result)
}

func TestHasOptionalWithoutDefault_Nested(t *testing.T) {
	expr := parseTypeExpr(t, `object({ inner = object({ name = optional(string) }) })`)
	result := HasOptionalWithoutDefault(expr)
	require.True(t, result)
}

func TestHasOptionalWithoutDefault_NestedWithDefault(t *testing.T) {
	expr := parseTypeExpr(t, `object({ inner = object({ name = optional(string, "") }) })`)
	result := HasOptionalWithoutDefault(expr)
	require.False(t, result)
}

func TestHasOptionalWithoutDefault_OptionalObject(t *testing.T) {
	expr := parseTypeExpr(t, `object({ inner = optional(object({ name = optional(string) })) })`)
	result := HasOptionalWithoutDefault(expr)
	require.True(t, result)
}
