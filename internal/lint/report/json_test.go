package report

import (
	"bytes"
	"encoding/json"
	"testing"

	"dangernoodle.io/terranoodle/internal/lint/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintJSON_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := PrintJSON(&buf, nil)
	require.NoError(t, err)

	// Should output "[]" with newline.
	assert.Equal(t, "[]\n", buf.String())
}

func TestPrintJSON_Output(t *testing.T) {
	errs := []validate.Error{
		{
			File:     "vars.tf",
			Variable: "test_var",
			Kind:     validate.MissingValidation,
			Detail:   "variable \"test_var\" lacks validation block",
			Severity: validate.SeverityError,
			Line:     5,
			Autofix:  true,
			Fix:      "add a validation block with condition and error_message",
		},
		{
			File:     "outputs.tf",
			Variable: "api_key",
			Kind:     validate.SensitiveOutput,
			Detail:   "output \"api_key\" should be marked sensitive",
			Severity: validate.SeverityError,
			Line:     10,
			Autofix:  true,
			Fix:      "add sensitive = true to this output block",
		},
	}

	var buf bytes.Buffer
	err := PrintJSON(&buf, errs)
	require.NoError(t, err)

	// Unmarshal and verify structure.
	var findings []JSONFinding
	err = json.Unmarshal(buf.Bytes(), &findings)
	require.NoError(t, err)

	require.Len(t, findings, 2)

	// First finding: MissingValidation (variable).
	assert.Equal(t, "missing-validation", findings[0].Rule)
	assert.Equal(t, "vars.tf", findings[0].File)
	assert.Equal(t, 5, findings[0].Line)
	assert.Equal(t, "variable", findings[0].Block)
	assert.Equal(t, "test_var", findings[0].Name)
	assert.Equal(t, "error", findings[0].Severity)
	assert.Equal(t, "variable \"test_var\" lacks validation block", findings[0].Detail)
	assert.True(t, findings[0].Autofix)
	assert.Equal(t, "add a validation block with condition and error_message", findings[0].Fix)

	// Second finding: SensitiveOutput (output).
	assert.Equal(t, "sensitive-output", findings[1].Rule)
	assert.Equal(t, "outputs.tf", findings[1].File)
	assert.Equal(t, 10, findings[1].Line)
	assert.Equal(t, "output", findings[1].Block)
	assert.Equal(t, "api_key", findings[1].Name)
	assert.Equal(t, "error", findings[1].Severity)
	assert.Equal(t, "output \"api_key\" should be marked sensitive", findings[1].Detail)
	assert.True(t, findings[1].Autofix)
	assert.Equal(t, "add sensitive = true to this output block", findings[1].Fix)
}

func TestPrintJSON_BlockTypeVariable(t *testing.T) {
	errs := []validate.Error{
		{
			File:     "vars.tf",
			Variable: "test_var",
			Kind:     validate.UnusedVariable,
			Detail:   "variable \"test_var\" is never used",
			Severity: validate.SeverityWarning,
			Line:     3,
		},
	}

	var buf bytes.Buffer
	err := PrintJSON(&buf, errs)
	require.NoError(t, err)

	var findings []JSONFinding
	err = json.Unmarshal(buf.Bytes(), &findings)
	require.NoError(t, err)

	assert.Len(t, findings, 1)
	assert.Equal(t, "variable", findings[0].Block)
}

func TestPrintJSON_BlockTypeOutput(t *testing.T) {
	errs := []validate.Error{
		{
			File:     "outputs.tf",
			Variable: "secret",
			Kind:     validate.SensitiveOutput,
			Detail:   "output \"secret\" should be marked sensitive",
			Severity: validate.SeverityError,
			Line:     15,
		},
	}

	var buf bytes.Buffer
	err := PrintJSON(&buf, errs)
	require.NoError(t, err)

	var findings []JSONFinding
	err = json.Unmarshal(buf.Bytes(), &findings)
	require.NoError(t, err)

	assert.Len(t, findings, 1)
	assert.Equal(t, "output", findings[0].Block)
}

func TestPrintJSON_MissingDescriptionVariable(t *testing.T) {
	errs := []validate.Error{
		{
			File:     "vars.tf",
			Variable: "env",
			Kind:     validate.MissingDescription,
			Detail:   "variable \"env\" lacks a description",
			Severity: validate.SeverityWarning,
			Line:     2,
			Autofix:  true,
			Fix:      "add a description attribute to this block",
		},
	}

	var buf bytes.Buffer
	err := PrintJSON(&buf, errs)
	require.NoError(t, err)

	var findings []JSONFinding
	err = json.Unmarshal(buf.Bytes(), &findings)
	require.NoError(t, err)

	assert.Len(t, findings, 1)
	assert.Equal(t, "variable", findings[0].Block)
	assert.Equal(t, "warning", findings[0].Severity)
}

func TestPrintJSON_MissingDescriptionOutput(t *testing.T) {
	errs := []validate.Error{
		{
			File:     "outputs.tf",
			Variable: "api_endpoint",
			Kind:     validate.MissingDescription,
			Detail:   "output \"api_endpoint\" lacks a description",
			Severity: validate.SeverityWarning,
			Line:     8,
		},
	}

	var buf bytes.Buffer
	err := PrintJSON(&buf, errs)
	require.NoError(t, err)

	var findings []JSONFinding
	err = json.Unmarshal(buf.Bytes(), &findings)
	require.NoError(t, err)

	assert.Len(t, findings, 1)
	assert.Equal(t, "output", findings[0].Block)
}

func TestPrintJSON_NonSnakeCaseVariable(t *testing.T) {
	errs := []validate.Error{
		{
			File:     "vars.tf",
			Variable: "myTestVar",
			Kind:     validate.NonSnakeCase,
			Detail:   "variable \"myTestVar\" should use snake_case",
			Severity: validate.SeverityError,
			Line:     12,
		},
	}

	var buf bytes.Buffer
	err := PrintJSON(&buf, errs)
	require.NoError(t, err)

	var findings []JSONFinding
	err = json.Unmarshal(buf.Bytes(), &findings)
	require.NoError(t, err)

	assert.Len(t, findings, 1)
	assert.Equal(t, "non-snake-case", findings[0].Rule)
	// Detail starts with "variable", so block should be inferred as "variable".
	assert.Equal(t, "variable", findings[0].Block)
}

func TestPrintJSON_OmitsEmptyFields(t *testing.T) {
	errs := []validate.Error{
		{
			File:     "vars.tf",
			Variable: "",
			Kind:     validate.MissingVersionsTF,
			Detail:   "versions.tf is missing",
			Severity: validate.SeverityError,
			Line:     1,
			Autofix:  false,
			Fix:      "",
		},
	}

	var buf bytes.Buffer
	err := PrintJSON(&buf, errs)
	require.NoError(t, err)

	var findings []JSONFinding
	err = json.Unmarshal(buf.Bytes(), &findings)
	require.NoError(t, err)

	assert.Len(t, findings, 1)
	// Verify that empty fields are omitted from JSON output.
	assert.Empty(t, findings[0].Block)
	assert.Empty(t, findings[0].Name)
	assert.Empty(t, findings[0].Fix)
	assert.False(t, findings[0].Autofix)
}
