package importer

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"dangernoodle.io/terranoodle/internal/state/resolver"
)

//go:embed templates/import_block.tmpl
var importTemplateFS embed.FS

var importTmpl = template.Must(
	template.New("import_block.tmpl").
		ParseFS(importTemplateFS, "templates/import_block.tmpl"),
)

type importData struct {
	Entries []resolver.ImportEntry
}

// GenerateImportsFile returns the contents of an imports.tf file containing
// one import block per entry.
func GenerateImportsFile(entries []resolver.ImportEntry) []byte {
	if len(entries) == 0 {
		return nil
	}
	var buf bytes.Buffer
	if err := importTmpl.Execute(&buf, importData{Entries: entries}); err != nil {
		panic("importer: render import template: " + err.Error())
	}
	return buf.Bytes()
}

// WriteImportsFile writes data to the specified path and returns the full path.
// If outputFlag is empty, defaults to <dir>/imports.tf.
// It returns an error if the file already exists and force is false.
func WriteImportsFile(dir string, outputFlag string, data []byte, force bool) (string, error) {
	path := outputFlag
	if path == "" {
		path = filepath.Join(dir, "imports.tf")
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return "", fmt.Errorf("imports.tf already exists — remove it or use --force to overwrite")
		}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("importer: write imports file: %w", err)
	}
	return path, nil
}

// RemoveImportsFile deletes the file at path if it exists.
func RemoveImportsFile(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
