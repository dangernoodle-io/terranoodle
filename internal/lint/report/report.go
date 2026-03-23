package report

import (
	"fmt"
	"io"

	"github.com/fatih/color"

	"dangernoodle.io/terra-tools/internal/lint/validate"
)

var (
	boldColor   = color.New(color.Bold)
	redColor    = color.New(color.FgRed)
	yellowColor = color.New(color.FgYellow)
)

func kindColor(kind validate.ErrorKind) *color.Color {
	switch kind {
	case validate.MissingRequired, validate.TypeMismatch:
		return redColor
	case validate.ExtraInput:
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
			fmt.Fprintf(w, "  %s: %s\n", kindColor(e.Kind).Sprint(e.Kind), e.Detail)
		}
	}

	summary := fmt.Sprintf("\n%d error(s) in %d file(s)", len(errs), len(order))
	fmt.Fprintln(w, redColor.Sprint(summary))
}
