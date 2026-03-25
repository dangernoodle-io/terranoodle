package report

import (
	"encoding/json"
	"io"
	"strings"

	"dangernoodle.io/terranoodle/internal/lint/validate"
)

// JSONFinding represents a single lint finding in JSON output.
type JSONFinding struct {
	Rule     string `json:"rule"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Block    string `json:"block,omitempty"`
	Name     string `json:"name,omitempty"`
	Severity string `json:"severity"`
	Detail   string `json:"detail"`
	Autofix  bool   `json:"autofix"`
	Fix      string `json:"fix,omitempty"`
}

// PrintJSON writes validation errors as a JSON array.
func PrintJSON(w io.Writer, errs []validate.Error) error {
	findings := make([]JSONFinding, len(errs))
	for i, e := range errs {
		block := blockType(e.Kind)
		// For MissingDescription and NonSnakeCase, infer block type from Detail.
		if block == "" && e.Variable != "" {
			if strings.HasPrefix(e.Detail, "variable") {
				block = "variable"
			} else if strings.HasPrefix(e.Detail, "output") {
				block = "output"
			}
		}

		findings[i] = JSONFinding{
			Rule:     validate.RuleName(e.Kind),
			File:     e.File,
			Line:     e.Line,
			Block:    block,
			Name:     e.Variable,
			Severity: severityString(e.Severity),
			Detail:   e.Detail,
			Autofix:  e.Autofix,
			Fix:      e.Fix,
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(findings)
}

// blockType returns the HCL block type for an ErrorKind.
func blockType(kind validate.ErrorKind) string {
	switch kind {
	case validate.MissingValidation, validate.UnusedVariable, validate.OptionalWithoutDefault, validate.SetStringType:
		return "variable"
	case validate.SensitiveOutput, validate.EmptyOutputsTF:
		return "output"
	default:
		return ""
	}
}

// severityString converts Severity to string.
func severityString(s validate.Severity) string {
	switch s {
	case validate.SeverityError:
		return "error"
	case validate.SeverityWarning:
		return "warning"
	default:
		return "error"
	}
}
