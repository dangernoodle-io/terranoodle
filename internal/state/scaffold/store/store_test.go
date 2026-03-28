package store_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	profileconfig "dangernoodle.io/terranoodle/internal/config"
	"dangernoodle.io/terranoodle/internal/state/config"
	"dangernoodle.io/terranoodle/internal/state/scaffold"
	"dangernoodle.io/terranoodle/internal/state/scaffold/store"
)

func TestStatePath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	path, err := store.StatePath("test-state")
	require.NoError(t, err)

	expected := filepath.Join(tmpDir, ".config", "terranoodle", "scaffold", "state", "test-state.yml")
	assert.Equal(t, expected, path)
}

func TestLoad_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonexistentPath := filepath.Join(tmpDir, "missing.yml")

	cfg, err := store.Load(nonexistentPath)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.Vars)
	assert.NotNil(t, cfg.Types)
	assert.NotNil(t, cfg.Resolvers)
	assert.Len(t, cfg.Types, 0)
}

func TestLoad_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "state.yml")

	// Create a test config file
	content := `
types:
  aws_instance:
    id: "{{ .id }}"
  aws_s3_bucket:
    id: "{{ .bucket }}"
vars:
  env: production
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))

	cfg, err := store.Load(filePath)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Len(t, cfg.Types, 2)
	assert.Equal(t, "{{ .id }}", cfg.Types["aws_instance"].ID)
	assert.Equal(t, "{{ .bucket }}", cfg.Types["aws_s3_bucket"].ID)
	assert.Equal(t, "production", cfg.Vars["env"])
}

func TestSave_CreatesDirectoryAndFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	filePath := filepath.Join(tmpDir, "subdir", "state.yml")

	cfg := &config.Config{
		Vars:      make(map[string]string),
		Types:     make(map[string]config.TypeMapping),
		Resolvers: make(map[string]config.Resolver),
	}
	cfg.Types["aws_instance"] = config.TypeMapping{ID: "{{ .id }}"}

	err := store.Save(filePath, cfg)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(filePath)
	assert.NoError(t, err)

	// Verify content
	loaded, err := store.Load(filePath)
	require.NoError(t, err)
	assert.Equal(t, "{{ .id }}", loaded.Types["aws_instance"].ID)
}

func TestSave_MergesWithExisting(t *testing.T) {
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "state.yml")

	// Create initial state
	existing := &config.Config{
		Vars:      make(map[string]string),
		Types:     make(map[string]config.TypeMapping),
		Resolvers: make(map[string]config.Resolver),
	}
	existing.Types["aws_instance"] = config.TypeMapping{ID: "id-v1"}
	existing.Types["aws_s3_bucket"] = config.TypeMapping{ID: "bucket-id"}

	require.NoError(t, store.Save(filePath, existing))

	// Merge in new config: override instance, add lambda
	incoming := &config.Config{
		Vars:      make(map[string]string),
		Types:     make(map[string]config.TypeMapping),
		Resolvers: make(map[string]config.Resolver),
	}
	incoming.Types["aws_instance"] = config.TypeMapping{ID: "id-v2"}
	incoming.Types["aws_lambda_function"] = config.TypeMapping{ID: "lambda-id"}

	require.NoError(t, store.Save(filePath, incoming))

	// Verify merge: instance overwritten, bucket preserved, lambda added
	loaded, err := store.Load(filePath)
	require.NoError(t, err)

	assert.Equal(t, "id-v2", loaded.Types["aws_instance"].ID)
	assert.Equal(t, "bucket-id", loaded.Types["aws_s3_bucket"].ID)
	assert.Equal(t, "lambda-id", loaded.Types["aws_lambda_function"].ID)
	assert.Len(t, loaded.Types, 3)
}

func TestPromptStateFile_DefaultAccepted(t *testing.T) {
	input := io.NopCloser(bytes.NewBufferString("\n"))
	output := &bytes.Buffer{}

	result, err := store.PromptStateFile(input, output, "aws", "default")
	require.NoError(t, err)
	assert.Equal(t, "default", result)
	assert.Contains(t, output.String(), "aws")
	assert.Contains(t, output.String(), "default")
}

func TestPromptStateFile_CustomInput(t *testing.T) {
	input := io.NopCloser(bytes.NewBufferString("custom-state\n"))
	output := &bytes.Buffer{}

	result, err := store.PromptStateFile(input, output, "google", "default")
	require.NoError(t, err)
	assert.Equal(t, "custom-state", result)
}

func TestSaveTypes_RoutesToCorrectFiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Config with two providers mapped to different state files
	globalCfg := &profileconfig.GlobalConfig{
		Profiles: map[string]profileconfig.Profile{
			"aws": {
				Scaffold: profileconfig.ScaffoldConfig{
					State:     "aws-state",
					Providers: []string{"aws"},
				},
			},
			"google": {
				Scaffold: profileconfig.ScaffoldConfig{
					State:     "google-state",
					Providers: []string{"google"},
				},
			},
		},
	}

	types := []scaffold.TypeInfo{
		{
			ResourceType: "aws_instance",
			Fields:       map[string]string{"id": "i-123"},
			IDTemplate:   "{{ .id }}",
		},
		{
			ResourceType: "google_compute_instance",
			Fields:       map[string]string{"id": "inst-123"},
			IDTemplate:   "{{ .id }}",
		},
	}

	input := io.NopCloser(bytes.NewBufferString(""))
	output := &bytes.Buffer{}

	err := store.SaveTypes(types, globalCfg, "", input, output)
	require.NoError(t, err)

	// Verify AWS types saved to aws-state.yml
	awsPath, err := store.StatePath("aws-state")
	require.NoError(t, err)
	awsCfg, err := store.Load(awsPath)
	require.NoError(t, err)
	assert.Equal(t, "{{ .id }}", awsCfg.Types["aws_instance"].ID)

	// Verify Google types saved to google-state.yml
	googlePath, err := store.StatePath("google-state")
	require.NoError(t, err)
	googleCfg, err := store.Load(googlePath)
	require.NoError(t, err)
	assert.Equal(t, "{{ .id }}", googleCfg.Types["google_compute_instance"].ID)

	// Verify no cross-provider pollution
	assert.NotContains(t, awsCfg.Types, "google_compute_instance")
	assert.NotContains(t, googleCfg.Types, "aws_instance")
}

func TestSaveTypes_SkipsTODO(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	globalCfg := &profileconfig.GlobalConfig{
		Profiles: map[string]profileconfig.Profile{
			"default": {
				Scaffold: profileconfig.ScaffoldConfig{
					State:     "default",
					Providers: []string{"aws"},
				},
			},
		},
	}

	types := []scaffold.TypeInfo{
		{
			ResourceType: "aws_instance",
			Fields:       map[string]string{"id": "i-123"},
			IDTemplate:   "{{ .id }}",
		},
		{
			ResourceType: "aws_custom",
			Fields:       map[string]string{"id": "custom-123"},
			IDTemplate:   "TODO",
		},
	}

	input := io.NopCloser(bytes.NewBufferString(""))
	output := &bytes.Buffer{}

	err := store.SaveTypes(types, globalCfg, "", input, output)
	require.NoError(t, err)

	// Verify only non-TODO types were saved
	defaultPath, err := store.StatePath("default")
	require.NoError(t, err)
	cfg, err := store.Load(defaultPath)
	require.NoError(t, err)

	assert.Len(t, cfg.Types, 1)
	assert.Contains(t, cfg.Types, "aws_instance")
	assert.NotContains(t, cfg.Types, "aws_custom")
}

func TestSaveTypes_PromptsForUnmapped(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Config with no providers defined
	globalCfg := &profileconfig.GlobalConfig{
		Profiles: make(map[string]profileconfig.Profile),
	}

	types := []scaffold.TypeInfo{
		{
			ResourceType: "acme_service",
			Fields:       map[string]string{"id": "svc-123"},
			IDTemplate:   "{{ .id }}",
		},
	}

	// User inputs "acme-state" when prompted
	input := io.NopCloser(bytes.NewBufferString("acme-state\n"))
	output := &bytes.Buffer{}

	err := store.SaveTypes(types, globalCfg, "", input, output)
	require.NoError(t, err)

	// Verify types saved to the prompted state file
	acmePath, err := store.StatePath("acme-state")
	require.NoError(t, err)
	cfg, err := store.Load(acmePath)
	require.NoError(t, err)

	assert.Equal(t, "{{ .id }}", cfg.Types["acme_service"].ID)
	assert.Contains(t, output.String(), "acme")
	assert.Contains(t, output.String(), "default")
}
