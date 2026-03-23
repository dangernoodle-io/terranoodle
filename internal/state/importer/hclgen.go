package importer

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"dangernoodle.io/terratools/internal/state/resolver"
)

// GenerateImportsFile returns the contents of an imports.tf file containing
// one import block per entry.
func GenerateImportsFile(entries []resolver.ImportEntry) []byte {
	var buf bytes.Buffer
	for i, e := range entries {
		if i > 0 {
			buf.WriteByte('\n')
		}
		fmt.Fprintf(&buf, "import {\n  to = %s\n  id = %q\n}\n", e.Address, e.ID)
	}
	return buf.Bytes()
}

// WriteImportsFile writes data to <dir>/imports.tf and returns the full path.
// It returns an error if the file already exists and force is false.
func WriteImportsFile(dir string, data []byte, force bool) (string, error) {
	path := filepath.Join(dir, "imports.tf")
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
