package catalog

import (
	"fmt"
	"os"
	"path/filepath"

	"dangernoodle.io/terratools/internal/hclutils"
	"dangernoodle.io/terratools/internal/output"
)

// Layout represents the catalog's directory structure.
type Layout struct {
	RootConfig string             // path to root/terragrunt-root.hcl
	ProjectDir string             // path to project/ directory
	Services   map[string]Service // service path -> Service (from project/ subdirectories)
	Config     *CatalogConfig     // optional catalog-level configuration
}

// Service represents a service in the catalog.
type Service struct {
	Path         string   // relative path from project dir (e.g., "looker", "region/redis")
	IsRegion     bool     // true if under region/ directory
	Dependencies []string // dependency block labels from terragrunt.hcl
}

// Walk scans a catalog directory and returns its Layout.
func Walk(catalogPath string) (*Layout, error) {
	// Verify root/terragrunt-root.hcl exists.
	rootConfig := filepath.Join(catalogPath, "root", "terragrunt-root.hcl")
	if _, err := os.Stat(rootConfig); err != nil {
		return nil, fmt.Errorf("catalog missing root/terragrunt-root.hcl at %s: %w", rootConfig, err)
	}

	// Verify project/ directory exists.
	projectDir := filepath.Join(catalogPath, "project")
	if info, err := os.Stat(projectDir); err != nil {
		return nil, fmt.Errorf("catalog missing project/ directory at %s: %w", projectDir, err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("catalog project/ is not a directory: %s", projectDir)
	}

	cfg, err := ParseCatalogConfig(catalogPath)
	if err != nil {
		return nil, fmt.Errorf("loading catalog config: %w", err)
	}
	if cfg.NameMustMatch != "" {
		output.Info("catalog config: name_must_match = %q", cfg.NameMustMatch)
	}
	if len(cfg.IgnoreDeps) > 0 {
		output.Info("catalog config: ignore_deps = %v", cfg.IgnoreDeps)
	}

	layout := &Layout{
		RootConfig: rootConfig,
		ProjectDir: projectDir,
		Services:   make(map[string]Service),
		Config:     cfg,
	}

	// Walk project/ recursively finding all directories with a terragrunt.hcl.
	err = filepath.WalkDir(projectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		// Skip the project/ root itself.
		if path == projectDir {
			return nil
		}

		// Check for terragrunt.hcl in this directory.
		tgFile := filepath.Join(path, "terragrunt.hcl")
		if _, statErr := os.Stat(tgFile); statErr != nil {
			return nil //nolint:nilerr // skip directories without terragrunt.hcl
		}

		// Compute relative path from project/.
		rel, err := filepath.Rel(projectDir, path)
		if err != nil {
			return fmt.Errorf("computing relative path for %s: %w", path, err)
		}

		// Determine if this is a regional service (under region/).
		isRegion := false
		parent := filepath.Dir(rel)
		if parent == "region" {
			isRegion = true
		}

		deps, err := hclutils.ParseDependencyLabels(tgFile)
		if err != nil {
			return fmt.Errorf("parsing dependencies for %s: %w", rel, err)
		}

		layout.Services[rel] = Service{
			Path:         rel,
			IsRegion:     isRegion,
			Dependencies: deps,
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking catalog project/ directory: %w", err)
	}

	return layout, nil
}
