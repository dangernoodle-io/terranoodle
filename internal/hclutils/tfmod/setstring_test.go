package tfmod

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasSetString_Nil(t *testing.T) {
	result := HasSetString(nil)
	require.False(t, result)
}

func TestHasSetString_SimpleType(t *testing.T) {
	expr := parseTypeExpr(t, "string")
	result := HasSetString(expr)
	require.False(t, result)
}

func TestHasSetString_SetString(t *testing.T) {
	expr := parseTypeExpr(t, "set(string)")
	result := HasSetString(expr)
	require.True(t, result)
}

func TestHasSetString_SetNumber(t *testing.T) {
	expr := parseTypeExpr(t, "set(number)")
	result := HasSetString(expr)
	require.False(t, result)
}

func TestHasSetString_ListString(t *testing.T) {
	expr := parseTypeExpr(t, "list(string)")
	result := HasSetString(expr)
	require.False(t, result)
}

func TestHasSetString_Nested(t *testing.T) {
	expr := parseTypeExpr(t, `object({ tags = set(string) })`)
	result := HasSetString(expr)
	require.True(t, result)
}

func TestHasSetString_ListOfSet(t *testing.T) {
	expr := parseTypeExpr(t, `list(set(string))`)
	result := HasSetString(expr)
	require.True(t, result)
}
