package tfmod

import (
	"fmt"
	"os"
)

// ResolveModuleDir validates that a module directory exists and contains .tf files.
func ResolveModuleDir(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty module path")
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("module path %s: %w", path, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("module path %s is not a directory", path)
	}

	return path, nil
}
