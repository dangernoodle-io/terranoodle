package validate

import (
	"os"
	"path/filepath"
	"strings"

	"dangernoodle.io/terranoodle/internal/hclutils"
)

// Dir validates terragrunt.hcl and/or terragrunt.stack.hcl in a single directory.
// If no terragrunt config is found, it falls back to validating Terraform module
// blocks in .tf files.
func Dir(dir string, opts ...Options) ([]Error, error) {
	var allErrors []Error
	foundTerragrunt := false

	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	for _, name := range []string{"terragrunt.hcl", "terragrunt.stack.hcl"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			continue
		}

		foundTerragrunt = true
		var errs []Error
		var toolErr error
		if name == "terragrunt.stack.hcl" {
			errs, toolErr = StackFile(path, opt)
		} else {
			errs, toolErr = File(path, opt)
		}
		if toolErr != nil {
			return nil, toolErr
		}
		allErrors = append(allErrors, errs...)
	}

	if !foundTerragrunt && hclutils.HasTFFiles(dir) {
		errs, err := TerraformDir(dir, opt)
		if err != nil {
			return nil, err
		}
		allErrors = append(allErrors, errs...)

		modErrs, modErr := ModuleDir(dir, opt)
		if modErr != nil {
			return nil, modErr
		}
		allErrors = append(allErrors, modErrs...)
	}

	return allErrors, nil
}

// WalkDir recursively finds terragrunt.hcl files under dir and validates each.
// Directories without terragrunt config but containing .tf files with module
// blocks are validated as standalone Terraform directories.
func WalkDir(dir string, opts ...Options) ([]Error, error) {
	var allErrors []Error

	// Track directories that have terragrunt config so we can skip TF fallback.
	visitedDirs := map[string]bool{}

	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and .terragrunt-cache
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == ".terragrunt-cache" {
				return filepath.SkipDir
			}
			if isExcludedDir(name, opt) {
				return filepath.SkipDir
			}
			return nil
		}

		errs, toolErr := validateFile(path, d.Name(), visitedDirs, opt)
		if toolErr != nil {
			return toolErr
		}
		allErrors = append(allErrors, errs...)
		return nil
	})

	return allErrors, err
}

// validateFile dispatches validation for a single file encountered during a
// directory walk. It returns any errors found and updates visitedDirs to
// prevent duplicate validation of the same directory.
func validateFile(path, name string, visitedDirs map[string]bool, opt Options) ([]Error, error) {
	if name == "terragrunt.hcl" || name == "terragrunt.stack.hcl" {
		dir := filepath.Dir(path)
		visitedDirs[dir] = true

		if name == "terragrunt.stack.hcl" {
			return StackFile(path, opt)
		}
		return File(path, opt)
	}

	// For .tf files in directories without terragrunt config, validate once per dir.
	if filepath.Ext(name) == ".tf" {
		tfDir := filepath.Dir(path)
		if !visitedDirs[tfDir] {
			// Mark the dir so we don't validate it again for subsequent .tf files.
			visitedDirs[tfDir] = true
			errs, err := TerraformDir(tfDir, opt)
			if err != nil {
				return nil, err
			}
			modErrs, modErr := ModuleDir(tfDir, opt)
			if modErr != nil {
				return nil, modErr
			}
			return append(errs, modErrs...), nil
		}
	}

	return nil, nil
}
