package tfmod

import (
	"fmt"
	"os"
	"path/filepath"
)

// ListTFFiles returns the base filenames of all .tf files in the given directory.
func ListTFFiles(moduleDir string) ([]string, error) {
	entries, err := os.ReadDir(moduleDir)
	if err != nil {
		return nil, fmt.Errorf("reading module dir %s: %w", moduleDir, err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".tf" {
			continue
		}
		files = append(files, entry.Name())
	}
	return files, nil
}
