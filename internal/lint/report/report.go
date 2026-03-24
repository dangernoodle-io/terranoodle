package report

import (
	"fmt"
	"io"

	"github.com/fatih/color"

	"dangernoodle.io/terranoodle/internal/lint/validate"
)

var (
	boldColor   = color.New(color.Bold)
	redColor    = color.New(color.FgRed)
	yellowColor = color.New(color.FgYellow)
)

func severityColor(severity validate.Severity) *color.Color {
	switch severity {
	case validate.SeverityError:
		return redColor
	case validate.SeverityWarning:
		return yellowColor
	default:
		return redColor
	}
}

// Print writes validation errors to w, grouped by file.
func Print(w io.Writer, errs []validate.Error) {
	if len(errs) == 0 {
		return
	}

	grouped := make(map[string][]validate.Error)
	var order []string

	for _, e := range errs {
		if _, seen := grouped[e.File]; !seen {
			order = append(order, e.File)
		}
		grouped[e.File] = append(grouped[e.File], e)
	}

	for i, file := range order {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s\n", boldColor.Sprint(file))
		for _, e := range grouped[file] {
			fmt.Fprintf(w, "  %s: %s\n", severityColor(e.Severity).Sprint(e.Kind), e.Detail)
		}
	}

	var errorCount, warningCount int
	for _, e := range errs {
		if e.Severity == validate.SeverityError {
			errorCount++
		} else {
			warningCount++
		}
	}

	var summary string
	if errorCount > 0 && warningCount > 0 {
		summary = fmt.Sprintf("\n%d error(s), %d warning(s) in %d file(s)", errorCount, warningCount, len(order))
	} else if warningCount > 0 {
		summary = fmt.Sprintf("\n%d warning(s) in %d file(s)", warningCount, len(order))
	} else {
		summary = fmt.Sprintf("\n%d error(s) in %d file(s)", errorCount, len(order))
	}

	summaryColor := redColor
	if errorCount == 0 && warningCount > 0 {
		summaryColor = yellowColor
	}
	fmt.Fprintln(w, summaryColor.Sprint(summary))
}
