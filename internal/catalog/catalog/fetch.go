package catalog

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"dangernoodle.io/terranoodle/internal/hclutils"
)

// Fetch retrieves a catalog from the given source (git URL or local path)
// and returns the local filesystem path plus a cleanup function. For remote
// sources the cleanup function removes the temporary clone directory. For
// local sources it is a no-op. The caller must call cleanup when done.
func Fetch(source string) (catalogPath string, cleanup func(), err error) {
	noop := func() {}

	if !hclutils.IsRemoteSource(source) {
		if _, err := os.Stat(source); err != nil {
			return "", noop, fmt.Errorf("catalog source %q not found: %w", source, err)
		}
		return source, noop, nil
	}

	// Remote source — clone to a temp directory.
	base, subdir, ref := hclutils.StripSubdir(source)

	tmpDir, err := os.MkdirTemp("", "terranoodle-catalog-*")
	if err != nil {
		return "", noop, fmt.Errorf("creating temp directory: %w", err)
	}

	cleanupFn := func() { os.RemoveAll(tmpDir) }

	// Strip git:: prefix for the actual git URL.
	gitURL := base
	if len(gitURL) > 5 && gitURL[:5] == "git::" {
		gitURL = gitURL[5:]
	}

	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, gitURL, tmpDir)

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		cleanupFn()
		return "", noop, fmt.Errorf("cloning catalog from %s: %w", gitURL, err)
	}

	if subdir != "" {
		return filepath.Join(tmpDir, subdir), cleanupFn, nil
	}
	return tmpDir, cleanupFn, nil
}
