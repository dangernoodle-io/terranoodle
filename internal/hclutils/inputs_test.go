package hclutils

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

// parseInputsExpr is a helper to extract the inputs attribute expression from HCL.
func parseInputsExpr(t *testing.T, hclSrc string) hcl.Expression {
	t.Helper()
	pos := hcl.Pos{Line: 1, Column: 1}
	file, diags := hclsyntax.ParseConfig([]byte(hclSrc), "test.hcl", pos)
	require.False(t, diags.HasErrors(), diags.Error())
	attrs, diags := file.Body.JustAttributes()
	require.False(t, diags.HasErrors(), diags.Error())
	attr, ok := attrs["inputs"]
	require.True(t, ok, "expected 'inputs' attribute")
	return attr.Expr
}

func TestExtractInputKeys(t *testing.T) {
	t.Run("plain object", func(t *testing.T) {
		hclSrc := `
inputs = {
  env  = "staging"
  name = "acme"
}
`
		expr := parseInputsExpr(t, hclSrc)
		keys, err := ExtractInputKeys(expr, nil)
		require.NoError(t, err)
		assert.Contains(t, keys, "env")
		assert.Contains(t, keys, "name")
		assert.Len(t, keys, 2)
	})

	t.Run("merge with objects", func(t *testing.T) {
		hclSrc := `
inputs = merge(
  { env = "staging" },
  { name = "acme" }
)
`
		expr := parseInputsExpr(t, hclSrc)
		keys, err := ExtractInputKeys(expr, nil)
		require.NoError(t, err)
		assert.Contains(t, keys, "env")
		assert.Contains(t, keys, "name")
		assert.Len(t, keys, 2)
	})

	t.Run("merge with overlapping keys", func(t *testing.T) {
		hclSrc := `
inputs = merge(
  { env = "staging", count = 1 },
  { env = "prod", name = "acme" }
)
`
		expr := parseInputsExpr(t, hclSrc)
		keys, err := ExtractInputKeys(expr, nil)
		require.NoError(t, err)
		assert.Contains(t, keys, "env")
		assert.Contains(t, keys, "count")
		assert.Contains(t, keys, "name")
	})

	t.Run("unsupported function", func(t *testing.T) {
		hclSrc := `
inputs = concat([])
`
		expr := parseInputsExpr(t, hclSrc)
		_, err := ExtractInputKeys(expr, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported function")
	})

	t.Run("empty object", func(t *testing.T) {
		hclSrc := `
inputs = {}
`
		expr := parseInputsExpr(t, hclSrc)
		keys, err := ExtractInputKeys(expr, nil)
		require.NoError(t, err)
		assert.Empty(t, keys)
	})

	t.Run("merge with three objects", func(t *testing.T) {
		hclSrc := `
inputs = merge(
  { a = 1 },
  { b = 2 },
  { c = 3 }
)
`
		expr := parseInputsExpr(t, hclSrc)
		keys, err := ExtractInputKeys(expr, nil)
		require.NoError(t, err)
		assert.Contains(t, keys, "a")
		assert.Contains(t, keys, "b")
		assert.Contains(t, keys, "c")
		assert.Len(t, keys, 3)
	})

	t.Run("object with numeric and boolean keys", func(t *testing.T) {
		hclSrc := `
inputs = {
  port     = 8080
  enabled  = true
  name     = "service"
}
`
		expr := parseInputsExpr(t, hclSrc)
		keys, err := ExtractInputKeys(expr, nil)
		require.NoError(t, err)
		assert.Contains(t, keys, "port")
		assert.Contains(t, keys, "enabled")
		assert.Contains(t, keys, "name")
		assert.Len(t, keys, 3)
	})
}

func TestExtractDepRefs(t *testing.T) {
	t.Run("merge with dependency refs", func(t *testing.T) {
		hclSrc := `
inputs = merge(
  { env = "staging" },
  dependency.alpha.outputs
)
`
		expr := parseInputsExpr(t, hclSrc)
		refs := ExtractDepRefs(expr)
		require.NotNil(t, refs)
		assert.Contains(t, refs, "alpha")
		assert.Len(t, refs, 1)
	})

	t.Run("merge with multiple dependency refs", func(t *testing.T) {
		hclSrc := `
inputs = merge(
  dependency.alpha.outputs,
  dependency.beta.outputs,
  { extra = "value" }
)
`
		expr := parseInputsExpr(t, hclSrc)
		refs := ExtractDepRefs(expr)
		require.NotNil(t, refs)
		assert.Contains(t, refs, "alpha")
		assert.Contains(t, refs, "beta")
		assert.Len(t, refs, 2)
	})

	t.Run("no deps in merge", func(t *testing.T) {
		hclSrc := `
inputs = merge(
  { env = "staging" },
  { name = "acme" }
)
`
		expr := parseInputsExpr(t, hclSrc)
		refs := ExtractDepRefs(expr)
		assert.Empty(t, refs)
	})

	t.Run("non-merge expression", func(t *testing.T) {
		hclSrc := `
inputs = {
  env = "staging"
}
`
		expr := parseInputsExpr(t, hclSrc)
		refs := ExtractDepRefs(expr)
		assert.Empty(t, refs)
	})

	t.Run("duplicate dependency refs are deduplicated", func(t *testing.T) {
		hclSrc := `
inputs = merge(
  dependency.alpha.outputs,
  dependency.alpha.outputs
)
`
		expr := parseInputsExpr(t, hclSrc)
		refs := ExtractDepRefs(expr)
		require.NotNil(t, refs)
		assert.Contains(t, refs, "alpha")
		assert.Len(t, refs, 1)
	})

	t.Run("dependency refs with objects", func(t *testing.T) {
		hclSrc := `
inputs = merge(
  { a = 1 },
  dependency.network.outputs,
  { b = 2 },
  dependency.database.outputs
)
`
		expr := parseInputsExpr(t, hclSrc)
		refs := ExtractDepRefs(expr)
		require.NotNil(t, refs)
		assert.Contains(t, refs, "network")
		assert.Contains(t, refs, "database")
		assert.Len(t, refs, 2)
	})
}

func TestExtractObjectKeys(t *testing.T) {
	t.Run("simple object", func(t *testing.T) {
		hclSrc := `
inputs = {
  env   = "staging"
  count = 1
}
`
		expr := parseInputsExpr(t, hclSrc)
		synExpr, ok := expr.(*hclsyntax.ObjectConsExpr)
		require.True(t, ok, "expected ObjectConsExpr")

		keys, err := ExtractObjectKeys(synExpr)
		require.NoError(t, err)
		assert.Contains(t, keys, "env")
		assert.Contains(t, keys, "count")
		assert.Len(t, keys, 2)
	})

	t.Run("object with string keys only", func(t *testing.T) {
		hclSrc := `
test_obj = {
  "key1" = "value1"
  "key2" = "value2"
}
`
		pos := hcl.Pos{Line: 1, Column: 1}
		file, diags := hclsyntax.ParseConfig([]byte(hclSrc), "test.hcl", pos)
		require.False(t, diags.HasErrors(), diags.Error())
		attrs, diags := file.Body.JustAttributes()
		require.False(t, diags.HasErrors(), diags.Error())

		attr, ok := attrs["test_obj"]
		require.True(t, ok)

		synExpr, ok := attr.Expr.(*hclsyntax.ObjectConsExpr)
		require.True(t, ok, "expected ObjectConsExpr")

		keys, err := ExtractObjectKeys(synExpr)
		require.NoError(t, err)
		assert.Contains(t, keys, "key1")
		assert.Contains(t, keys, "key2")
		assert.Len(t, keys, 2)
	})

	t.Run("empty object", func(t *testing.T) {
		hclSrc := `
inputs = {}
`
		expr := parseInputsExpr(t, hclSrc)
		synExpr, ok := expr.(*hclsyntax.ObjectConsExpr)
		require.True(t, ok, "expected ObjectConsExpr")

		keys, err := ExtractObjectKeys(synExpr)
		require.NoError(t, err)
		assert.Empty(t, keys)
	})

	t.Run("object with various value types", func(t *testing.T) {
		hclSrc := `
inputs = {
  string_key = "value"
  number_key = 42
  bool_key   = true
  list_key   = [1, 2, 3]
}
`
		expr := parseInputsExpr(t, hclSrc)
		synExpr, ok := expr.(*hclsyntax.ObjectConsExpr)
		require.True(t, ok, "expected ObjectConsExpr")

		keys, err := ExtractObjectKeys(synExpr)
		require.NoError(t, err)
		assert.Contains(t, keys, "string_key")
		assert.Contains(t, keys, "number_key")
		assert.Contains(t, keys, "bool_key")
		assert.Contains(t, keys, "list_key")
		assert.Len(t, keys, 4)
	})
}

func TestExtractInputKeysWithContext(t *testing.T) {
	t.Run("merge with evaluated traversal", func(t *testing.T) {
		hclSrc := `
inputs = merge(
  { env = "staging" },
  include.root.locals
)
`
		expr := parseInputsExpr(t, hclSrc)

		// Create a context with include.root.locals
		ctx := &hcl.EvalContext{
			Variables: map[string]cty.Value{
				"include": cty.ObjectVal(map[string]cty.Value{
					"root": cty.ObjectVal(map[string]cty.Value{
						"locals": cty.ObjectVal(map[string]cty.Value{
							"project": cty.StringVal("acme"),
						}),
					}),
				}),
			},
		}

		keys, err := ExtractInputKeys(expr, ctx)
		require.NoError(t, err)
		assert.Contains(t, keys, "env")
		assert.Contains(t, keys, "project")
	})

	t.Run("merge without context for traversal", func(t *testing.T) {
		hclSrc := `
inputs = merge(
  { env = "staging" },
  include.root.locals
)
`
		expr := parseInputsExpr(t, hclSrc)

		// No context provided; traversal should be skipped
		keys, err := ExtractInputKeys(expr, nil)
		require.NoError(t, err)
		assert.Contains(t, keys, "env")
		// include.root.locals won't be evaluated without context
	})
}
