package output

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

var (
	errorColor   = color.New(color.FgRed)
	warnColor    = color.New(color.FgYellow)
	successColor = color.New(color.FgGreen)
	boldColor    = color.New(color.Bold)
	cyanColor    = color.New(color.FgCyan)
	dimColor     = color.New(color.Faint)
	itemColor    = color.New(color.FgGreen)
)

func Error(format string, a ...interface{}) {
	fmt.Fprintln(os.Stderr, errorColor.Sprintf(format, a...))
}

func Warn(format string, a ...interface{}) {
	fmt.Fprintln(os.Stderr, warnColor.Sprintf(format, a...))
}

func Success(format string, a ...interface{}) {
	fmt.Fprintln(os.Stdout, successColor.Sprintf(format, a...))
}

func Info(format string, a ...interface{}) {
	fmt.Fprintln(os.Stdout, fmt.Sprintf(format, a...))
}

func Bold(format string, a ...interface{}) string {
	return boldColor.Sprintf(format, a...)
}

// Cyan returns a cyan-formatted string (for resource addresses).
func Cyan(format string, a ...interface{}) string {
	return cyanColor.Sprintf(format, a...)
}

// DryRun prints a dimmed dry-run command preview line.
func DryRun(format string, a ...interface{}) {
	fmt.Fprintln(os.Stdout, dimColor.Sprintf(format, a...))
}

// Item prints a per-item success indicator.
func Item(format string, a ...interface{}) {
	fmt.Fprintf(os.Stdout, "  %s %s\n", itemColor.Sprint("✓"), fmt.Sprintf(format, a...))
}

// Disable turns off all color output.
func Disable() {
	color.NoColor = true
}
