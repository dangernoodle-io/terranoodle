package hclutils

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveSource takes a raw source string from a terragrunt config and the
// path to the config file, and returns the local filesystem path to the module.
// For git sources, it scans .terragrunt-cache/ for a cached copy.
func ResolveSource(source string, configPath string) string {
	if source == "" {
		return ""
	}

	configDir := filepath.Dir(configPath)

	// Handle tfr:// registry sources — not supported
	if strings.HasPrefix(source, "tfr://") {
		return ""
	}

	// Git/remote sources — scan .terragrunt-cache
	if IsRemoteSource(source) {
		_, subdir, _ := StripSubdir(source)
		return scanCache(configDir, subdir)
	}

	// Local path — resolve relative to config file location
	if filepath.IsAbs(source) {
		return source
	}

	return filepath.Join(configDir, source)
}

// IsRemoteSource reports whether source is a remote (non-local) module reference.
func IsRemoteSource(source string) bool {
	return strings.HasPrefix(source, "git::") ||
		strings.HasPrefix(source, "github.com/") ||
		strings.HasPrefix(source, "gitlab.com/") ||
		strings.Contains(source, "://")
}

// scanCache looks for a module in .terragrunt-cache/<hash1>/<hash2>/[subdir].
// When multiple cached versions exist, returns the most recently modified one.
func scanCache(configDir string, subdir string) string {
	cacheDir := filepath.Join(configDir, ".terragrunt-cache")

	hash1Entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return ""
	}

	var best string
	var bestTime int64

	for _, h1 := range hash1Entries {
		if !h1.IsDir() {
			continue
		}

		h1Path := filepath.Join(cacheDir, h1.Name())
		hash2Entries, err := os.ReadDir(h1Path)
		if err != nil {
			continue
		}

		for _, h2 := range hash2Entries {
			if !h2.IsDir() {
				continue
			}

			candidate := filepath.Join(h1Path, h2.Name())
			if subdir != "" {
				candidate = filepath.Join(candidate, subdir)
			}

			if !HasTFFiles(candidate) {
				continue
			}

			info, err := h2.Info()
			if err != nil {
				continue
			}
			if t := info.ModTime().UnixNano(); best == "" || t > bestTime {
				best = candidate
				bestTime = t
			}
		}
	}

	return best
}

// HasTFFiles reports whether dir contains at least one .tf file.
func HasTFFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".tf" {
			return true
		}
	}
	return false
}

// StripSubdir splits a module source into base URL and subdirectory.
// For example: "git::https://example.com/repo.git//subdir?ref=v1" → ("git::https://example.com/repo.git", "subdir", "v1").
func StripSubdir(source string) (base, subdir, ref string) {
	// Strip ?ref= query parameter
	if idx := strings.Index(source, "?ref="); idx != -1 {
		ref = source[idx+5:]
		source = source[:idx]
	}

	// Split on // for subdirectory — skip the protocol prefix (git::https://)
	searchFrom := 0
	if protoIdx := strings.Index(source, "://"); protoIdx != -1 {
		searchFrom = protoIdx + 3
	}
	if subIdx := strings.Index(source[searchFrom:], "//"); subIdx != -1 {
		splitAt := searchFrom + subIdx
		base = source[:splitAt]
		subdir = source[splitAt+2:]
		return
	}

	base = source
	return
}
