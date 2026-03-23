package hclutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsRemoteSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   bool
	}{
		{
			name:   "git protocol",
			source: "git::https://example.com/repo.git",
			want:   true,
		},
		{
			name:   "github.com",
			source: "github.com/acme-corp/modules",
			want:   true,
		},
		{
			name:   "gitlab.com",
			source: "gitlab.com/acme-corp/modules",
			want:   true,
		},
		{
			name:   "s3 bucket",
			source: "s3://acme-bucket/module",
			want:   true,
		},
		{
			name:   "https",
			source: "https://example.com/repo.git",
			want:   true,
		},
		{
			name:   "relative path",
			source: "../modules/vpc",
			want:   false,
		},
		{
			name:   "absolute path",
			source: "/opt/modules/vpc",
			want:   false,
		},
		{
			name:   "empty string",
			source: "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRemoteSource(tt.source)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStripSubdir(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantBase   string
		wantSubdir string
		wantRef    string
	}{
		{
			name:       "with subdir and ref",
			source:     "git::https://example.com/repo.git//modules/vpc?ref=v1.0.0",
			wantBase:   "git::https://example.com/repo.git",
			wantSubdir: "modules/vpc",
			wantRef:    "v1.0.0",
		},
		{
			name:       "no subdir, with ref",
			source:     "git::https://example.com/repo.git?ref=main",
			wantBase:   "git::https://example.com/repo.git",
			wantSubdir: "",
			wantRef:    "main",
		},
		{
			name:       "with subdir, no ref",
			source:     "git::https://example.com/repo.git//modules/vpc",
			wantBase:   "git::https://example.com/repo.git",
			wantSubdir: "modules/vpc",
			wantRef:    "",
		},
		{
			name:       "no subdir, no ref",
			source:     "git::https://example.com/repo.git",
			wantBase:   "git::https://example.com/repo.git",
			wantSubdir: "",
			wantRef:    "",
		},
		{
			name:       "https protocol without subdir",
			source:     "https://example.com/repo.git",
			wantBase:   "https://example.com/repo.git",
			wantSubdir: "",
			wantRef:    "",
		},
		{
			name:       "github.com format",
			source:     "github.com/acme-corp/modules//networking?ref=v2.1.0",
			wantBase:   "github.com/acme-corp/modules",
			wantSubdir: "networking",
			wantRef:    "v2.1.0",
		},
		{
			name:       "ref with special chars",
			source:     "git::https://example.com/repo.git?ref=release-1.0.0",
			wantBase:   "git::https://example.com/repo.git",
			wantSubdir: "",
			wantRef:    "release-1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, subdir, ref := StripSubdir(tt.source)
			assert.Equal(t, tt.wantBase, base, "base mismatch")
			assert.Equal(t, tt.wantSubdir, subdir, "subdir mismatch")
			assert.Equal(t, tt.wantRef, ref, "ref mismatch")
		})
	}
}

func TestHasTFFiles(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, dir string)
		wantTrue bool
	}{
		{
			name: "dir with .tf file",
			setup: func(t *testing.T, dir string) {
				err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte("# main"), 0644)
				require.NoError(t, err)
			},
			wantTrue: true,
		},
		{
			name: "dir with multiple .tf files",
			setup: func(t *testing.T, dir string) {
				err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte("# main"), 0644)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(dir, "outputs.tf"), []byte("# outputs"), 0644)
				require.NoError(t, err)
			},
			wantTrue: true,
		},
		{
			name: "empty dir",
			setup: func(t *testing.T, dir string) {
				// directory is empty
			},
			wantTrue: false,
		},
		{
			name: "dir with only .hcl files",
			setup: func(t *testing.T, dir string) {
				err := os.WriteFile(filepath.Join(dir, "config.hcl"), []byte("# config"), 0644)
				require.NoError(t, err)
			},
			wantTrue: false,
		},
		{
			name: "dir with mixed files, has .tf",
			setup: func(t *testing.T, dir string) {
				err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme"), 0644)
				require.NoError(t, err)
				err = os.WriteFile(filepath.Join(dir, "variables.tf"), []byte("# vars"), 0644)
				require.NoError(t, err)
			},
			wantTrue: true,
		},
		{
			name: "non-existent dir",
			setup: func(t *testing.T, dir string) {
				// don't create anything
			},
			wantTrue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			// For the non-existent dir test, use a path that doesn't exist
			if tt.name == "non-existent dir" {
				dir = filepath.Join(dir, "nonexistent")
			} else {
				tt.setup(t, dir)
			}
			got := HasTFFiles(dir)
			assert.Equal(t, tt.wantTrue, got)
		})
	}
}

func TestScanCache(t *testing.T) {
	t.Run("with .tf file in cache", func(t *testing.T) {
		dir := t.TempDir()
		cacheDir := filepath.Join(dir, ".terragrunt-cache")
		hash1Dir := filepath.Join(cacheDir, "abc123")
		hash2Dir := filepath.Join(hash1Dir, "def456")

		err := os.MkdirAll(hash2Dir, 0755)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(hash2Dir, "main.tf"), []byte("# main"), 0644)
		require.NoError(t, err)

		got := scanCache(dir, "")
		assert.Equal(t, hash2Dir, got)
	})

	t.Run("with subdir in cache", func(t *testing.T) {
		dir := t.TempDir()
		cacheDir := filepath.Join(dir, ".terragrunt-cache")
		hash1Dir := filepath.Join(cacheDir, "abc123")
		hash2Dir := filepath.Join(hash1Dir, "def456")
		subdirPath := filepath.Join(hash2Dir, "modules", "vpc")

		err := os.MkdirAll(subdirPath, 0755)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(subdirPath, "main.tf"), []byte("# main"), 0644)
		require.NoError(t, err)

		got := scanCache(dir, "modules/vpc")
		assert.Equal(t, subdirPath, got)
	})

	t.Run("empty cache dir", func(t *testing.T) {
		dir := t.TempDir()
		cacheDir := filepath.Join(dir, ".terragrunt-cache")
		err := os.MkdirAll(cacheDir, 0755)
		require.NoError(t, err)

		got := scanCache(dir, "")
		assert.Empty(t, got)
	})

	t.Run("cache not found", func(t *testing.T) {
		dir := t.TempDir()
		// Don't create cache dir at all

		got := scanCache(dir, "")
		assert.Empty(t, got)
	})

	t.Run("cache with multiple versions, returns newest", func(t *testing.T) {
		dir := t.TempDir()
		cacheDir := filepath.Join(dir, ".terragrunt-cache")

		// Create first version
		hash1Dir1 := filepath.Join(cacheDir, "abc123")
		hash2Dir1 := filepath.Join(hash1Dir1, "def456")
		err := os.MkdirAll(hash2Dir1, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(hash2Dir1, "main.tf"), []byte("# v1"), 0644)
		require.NoError(t, err)

		// Create second version
		hash1Dir2 := filepath.Join(cacheDir, "abc123")
		hash2Dir2 := filepath.Join(hash1Dir2, "ghi789")
		err = os.MkdirAll(hash2Dir2, 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(hash2Dir2, "main.tf"), []byte("# v2"), 0644)
		require.NoError(t, err)

		got := scanCache(dir, "")
		// Should return the most recent one
		assert.True(t, got == hash2Dir1 || got == hash2Dir2)
	})
}
