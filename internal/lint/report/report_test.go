package report

import (
	"bytes"
	"testing"

	"dangernoodle.io/terranoodle/internal/lint/validate"
	"github.com/stretchr/testify/assert"
)

func TestPrint_Empty(t *testing.T) {
	var buf bytes.Buffer
	Print(&buf, nil)
	assert.Empty(t, buf.String())
}

func TestPrint_GroupedByFile(t *testing.T) {
	errs := []validate.Error{
		{File: "/a/terragrunt.hcl", Variable: "foo", Kind: validate.MissingRequired, Detail: `variable "foo" is required (no default) but not provided in inputs`, Severity: validate.SeverityError},
		{File: "/a/terragrunt.hcl", Variable: "bar", Kind: validate.ExtraInput, Detail: `input "bar" has no matching variable in module`, Severity: validate.SeverityError},
		{File: "/b/terragrunt.hcl", Variable: "baz", Kind: validate.MissingRequired, Detail: `variable "baz" is required (no default) but not provided in inputs`, Severity: validate.SeverityError},
	}

	var buf bytes.Buffer
	Print(&buf, errs)

	out := buf.String()
	assert.Contains(t, out, "/a/terragrunt.hcl")
	assert.Contains(t, out, "/b/terragrunt.hcl")
	assert.Contains(t, out, "missing required input")
	assert.Contains(t, out, "extra input")
	assert.Contains(t, out, "3 error(s) in 2 file(s)")
}

func TestPrint_WarningsOnly(t *testing.T) {
	errs := []validate.Error{
		{File: "/a/terragrunt.hcl", Kind: validate.ExtraInput, Detail: "test warning", Severity: validate.SeverityWarning},
	}

	var buf bytes.Buffer
	Print(&buf, errs)

	out := buf.String()
	assert.Contains(t, out, "1 warning(s) in 1 file(s)")
	assert.NotContains(t, out, "error(s)")
}

func TestPrint_Mixed(t *testing.T) {
	errs := []validate.Error{
		{File: "/a/terragrunt.hcl", Kind: validate.MissingRequired, Detail: "test error", Severity: validate.SeverityError},
		{File: "/a/terragrunt.hcl", Kind: validate.ExtraInput, Detail: "test warning", Severity: validate.SeverityWarning},
	}

	var buf bytes.Buffer
	Print(&buf, errs)

	out := buf.String()
	assert.Contains(t, out, "1 error(s), 1 warning(s) in 1 file(s)")
}

func TestPrint_ErrorsOnly(t *testing.T) {
	errs := []validate.Error{
		{File: "/a/terragrunt.hcl", Kind: validate.MissingRequired, Detail: "test error", Severity: validate.SeverityError},
	}

	var buf bytes.Buffer
	Print(&buf, errs)

	out := buf.String()
	assert.Contains(t, out, "1 error(s) in 1 file(s)")
	assert.NotContains(t, out, "warning(s)")
}
