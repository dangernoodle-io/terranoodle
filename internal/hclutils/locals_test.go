package hclutils

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

// parseLocalsBody is a helper to create an hcl.Body from a locals block.
func parseLocalsBody(t *testing.T, src string) hcl.Body {
	t.Helper()
	file, diags := hclsyntax.ParseConfig([]byte(src), "test.hcl", hcl.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors(), diags.Error())
	content, _, diags := file.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "locals"}},
	})
	require.False(t, diags.HasErrors(), diags.Error())
	require.NotEmpty(t, content.Blocks, "no locals block found")
	return content.Blocks[0].Body
}

func TestEvalLocals(t *testing.T) {
	t.Run("simple values", func(t *testing.T) {
		src := `
locals {
  env   = "staging"
  count = 1
  enabled = true
}
`
		body := parseLocalsBody(t, src)
		ctx := &hcl.EvalContext{Variables: map[string]cty.Value{}}

		result := EvalLocals(body, ctx)
		require.NotNil(t, result)
		assert.Len(t, result, 3)

		assert.Equal(t, cty.StringVal("staging"), result["env"])
		assert.Equal(t, cty.Number, result["count"].Type())
		assert.Equal(t, cty.BoolVal(true), result["enabled"])
	})

	t.Run("self-reference", func(t *testing.T) {
		src := `
locals {
  first = "acme"
  last  = "corp"
  full  = "${local.first}-${local.last}"
}
`
		body := parseLocalsBody(t, src)
		ctx := &hcl.EvalContext{Variables: map[string]cty.Value{}}

		result := EvalLocals(body, ctx)
		require.NotNil(t, result)
		assert.Len(t, result, 3)

		assert.Equal(t, cty.StringVal("acme"), result["first"])
		assert.Equal(t, cty.StringVal("corp"), result["last"])
		assert.Equal(t, cty.StringVal("acme-corp"), result["full"])
	})

	t.Run("unresolvable reference", func(t *testing.T) {
		src := `
locals {
  bad = some_undefined_var
}
`
		body := parseLocalsBody(t, src)
		ctx := &hcl.EvalContext{Variables: map[string]cty.Value{}}

		result := EvalLocals(body, ctx)
		require.NotNil(t, result)
		assert.Len(t, result, 1)

		// Unresolvable references should be set to DynamicVal
		assert.Equal(t, cty.DynamicVal, result["bad"])
	})

	t.Run("empty locals block", func(t *testing.T) {
		src := `
locals {
}
`
		body := parseLocalsBody(t, src)
		ctx := &hcl.EvalContext{Variables: map[string]cty.Value{}}

		result := EvalLocals(body, ctx)
		assert.Empty(t, result)
	})

	t.Run("multi-level self-references", func(t *testing.T) {
		src := `
locals {
  a = "hello"
  b = "${local.a}-world"
  c = "${local.b}!"
}
`
		body := parseLocalsBody(t, src)
		ctx := &hcl.EvalContext{Variables: map[string]cty.Value{}}

		result := EvalLocals(body, ctx)
		require.NotNil(t, result)
		assert.Len(t, result, 3)

		assert.Equal(t, cty.StringVal("hello"), result["a"])
		assert.Equal(t, cty.StringVal("hello-world"), result["b"])
		assert.Equal(t, cty.StringVal("hello-world!"), result["c"])
	})

	t.Run("numeric and boolean values", func(t *testing.T) {
		src := `
locals {
  port          = 8080
  enable_tls    = true
  timeout_secs  = 30.5
  max_retries   = 3
}
`
		body := parseLocalsBody(t, src)
		ctx := &hcl.EvalContext{Variables: map[string]cty.Value{}}

		result := EvalLocals(body, ctx)
		require.NotNil(t, result)
		assert.Len(t, result, 4)

		assert.Equal(t, cty.Number, result["port"].Type())
		assert.Equal(t, cty.BoolVal(true), result["enable_tls"])
		assert.Equal(t, cty.Number, result["max_retries"].Type())
		assert.Equal(t, cty.Number, result["timeout_secs"].Type())
	})

	t.Run("list and map values", func(t *testing.T) {
		src := `
locals {
  tags = ["env", "team"]
  labels = {
    environment = "staging"
    owner       = "acme-corp"
  }
}
`
		body := parseLocalsBody(t, src)
		ctx := &hcl.EvalContext{Variables: map[string]cty.Value{}}

		result := EvalLocals(body, ctx)
		require.NotNil(t, result)
		assert.Len(t, result, 2)

		// Verify list was parsed
		assert.NotNil(t, result["tags"])
		assert.NotNil(t, result["labels"])
	})

	t.Run("context variables preserved", func(t *testing.T) {
		src := `
locals {
  name = var_from_context
}
`
		body := parseLocalsBody(t, src)
		ctx := &hcl.EvalContext{
			Variables: map[string]cty.Value{
				"var_from_context": cty.StringVal("test-value"),
			},
		}

		result := EvalLocals(body, ctx)
		require.NotNil(t, result)
		assert.Len(t, result, 1)
		assert.Equal(t, cty.StringVal("test-value"), result["name"])
	})
}
