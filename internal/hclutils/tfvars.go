package hclutils

import (
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// ParseTfVarKeys reads tfvars files and returns the set of variable names defined in them.
// Missing files are silently skipped (optional_var_files semantics).
func ParseTfVarKeys(paths []string) map[string]bool {
	keys := make(map[string]bool)

	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			// Skip missing or unreadable files (optional_var_files semantics)
			continue
		}

		file, diags := hclsyntax.ParseConfig(src, path, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			// Skip files that don't parse
			continue
		}

		attrs, diags := file.Body.JustAttributes()
		if diags.HasErrors() {
			// Skip if we can't read attributes
			continue
		}

		// Collect all attribute names
		for name := range attrs {
			keys[name] = true
		}
	}

	return keys
}
