package output

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestError(t *testing.T) {
	// Disable colors for consistent test output
	oldNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = oldNoColor })

	output := captureStderr(t, func() {
		Error("error: %s", "test message")
	})

	assert.Contains(t, output, "error: test message")
}

func TestWarn(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = oldNoColor })

	output := captureStderr(t, func() {
		Warn("warning: %s", "test warning")
	})

	assert.Contains(t, output, "warning: test warning")
}

func TestSuccess(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = oldNoColor })

	output := captureStdout(t, func() {
		Success("success: %s", "operation complete")
	})

	assert.Contains(t, output, "success: operation complete")
}

func TestInfo(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = oldNoColor })

	output := captureStdout(t, func() {
		Info("info: %s", "informational message")
	})

	assert.Contains(t, output, "info: informational message")
}

func TestBold(t *testing.T) {
	oldNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = oldNoColor })

	result := Bold("bold %s", "text")
	assert.Contains(t, result, "bold text")
}

func TestDisable(t *testing.T) {
	oldNoColor := color.NoColor
	t.Cleanup(func() { color.NoColor = oldNoColor })

	color.NoColor = false
	Disable()
	assert.True(t, color.NoColor)
}
