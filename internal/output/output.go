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

// Disable turns off all color output.
func Disable() {
	color.NoColor = true
}
