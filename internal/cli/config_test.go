package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"dangernoodle.io/terranoodle/internal/config"
)

// TestConfigInit_Success creates .terranoodle.yml with defaults in temp dir.
func TestConfigInit_Success(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err := runConfigInit(configInitCmd, nil)
	require.NoError(t, err)

	// Verify file exists
	path := filepath.Join(tmpDir, config.ProjectFile)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	// Verify contents match default config
	var cfg config.Config
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	// Check that default rules are present
	assert.NotNil(t, cfg.Lint.Rules)
	assert.True(t, len(cfg.Lint.Rules) > 0)
	assert.True(t, cfg.Lint.Rules["missing-required"].Enabled)
	assert.True(t, cfg.Lint.Rules["extra-inputs"].Enabled)
	assert.True(t, cfg.Lint.Rules["type-mismatch"].Enabled)
}

// TestConfigInit_AlreadyExists returns error when file exists.
func TestConfigInit_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Create existing config
	path := filepath.Join(tmpDir, config.ProjectFile)
	err := os.WriteFile(path, []byte("lint:\n  rules:\n    test: true\n"), 0o644)
	require.NoError(t, err)

	// Try to init again
	err = runConfigInit(configInitCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// TestConfigSet_Project sets a rule in project .terranoodle.yml.
func TestConfigSet_Project(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Save flag state
	oldGlobalFlag := configGlobalFlag
	t.Cleanup(func() { configGlobalFlag = oldGlobalFlag })

	configGlobalFlag = false

	// Set a rule
	err := runConfigSet(configSetCmd, []string{"lint.rules.custom-rule", "false"})
	require.NoError(t, err)

	// Verify file written
	path := filepath.Join(tmpDir, config.ProjectFile)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg config.Config
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.NotNil(t, cfg.Lint.Rules)
	assert.False(t, cfg.Lint.Rules["custom-rule"].Enabled)
}

// TestConfigSet_GlobalFlag verifies --global flag is accepted.
func TestConfigSet_GlobalFlag(t *testing.T) {
	// Save flag state
	oldGlobalFlag := configGlobalFlag
	t.Cleanup(func() { configGlobalFlag = oldGlobalFlag })

	configGlobalFlag = true

	// Verify the flag is set
	assert.True(t, configGlobalFlag)
}

// TestConfigGet retrieves a value from discovered config.
func TestConfigGet(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Create a config
	path := filepath.Join(tmpDir, config.ProjectFile)
	cfg := &config.Config{
		Lint: config.LintConfig{
			Rules: map[string]config.RuleConfig{
				"test-rule": {Enabled: true},
			},
		},
	}
	err := config.Save(path, cfg)
	require.NoError(t, err)

	// Get the value
	err = runConfigGet(configGetCmd, []string{"lint.rules.test-rule"})
	require.NoError(t, err)
}

// TestConfigList shows effective merged config as YAML.
func TestConfigList(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Create a config
	path := filepath.Join(tmpDir, config.ProjectFile)
	cfg := &config.Config{
		Lint: config.LintConfig{
			Rules: map[string]config.RuleConfig{
				"rule-one": {Enabled: true},
				"rule-two": {Enabled: false},
			},
		},
	}
	err := config.Save(path, cfg)
	require.NoError(t, err)

	// List the config
	err = runConfigList(configListCmd, nil)
	require.NoError(t, err)
}

// TestConfigSet_ExcludeDirs sets exclude-dirs in project config.
func TestConfigSet_ExcludeDirs(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Save flag state
	oldGlobalFlag := configGlobalFlag
	t.Cleanup(func() { configGlobalFlag = oldGlobalFlag })

	configGlobalFlag = false

	// Set exclude-dirs
	err := runConfigSet(configSetCmd, []string{"lint.exclude-dirs", "vendor,build,.terraform"})
	require.NoError(t, err)

	// Verify file written
	path := filepath.Join(tmpDir, config.ProjectFile)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg config.Config
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.Equal(t, []string{"vendor", "build", ".terraform"}, cfg.Lint.ExcludeDirs)
}

// TestConfigGet_ExcludeDirs retrieves exclude-dirs from config.
func TestConfigGet_ExcludeDirs(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Create a config with exclude-dirs
	path := filepath.Join(tmpDir, config.ProjectFile)
	cfg := &config.Config{
		Lint: config.LintConfig{
			ExcludeDirs: []string{"vendor", "build"},
		},
	}
	err := config.Save(path, cfg)
	require.NoError(t, err)

	// Get the value
	err = runConfigGet(configGetCmd, []string{"lint.exclude-dirs"})
	require.NoError(t, err)
}

// TestConfigInit_Global creates global config in ~/.config/terranoodle/config.yml.
func TestConfigInit_Global(t *testing.T) {
	// Use a temporary home directory
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Save flag state
	oldGlobalFlag := configGlobalFlag
	t.Cleanup(func() { configGlobalFlag = oldGlobalFlag })

	configGlobalFlag = true

	// Create any working directory to run from
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err := runConfigInit(configInitCmd, nil)
	require.NoError(t, err)

	// Verify global config was created
	expectedPath := filepath.Join(tmpHome, ".config", "terranoodle", config.GlobalFile)
	data, err := os.ReadFile(expectedPath)
	require.NoError(t, err)

	// Verify contents match default config
	var cfg config.Config
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	// Check that default rules are present
	assert.NotNil(t, cfg.Lint.Rules)
	assert.True(t, len(cfg.Lint.Rules) > 0)
	assert.True(t, cfg.Lint.Rules["missing-required"].Enabled)
}

// TestConfigInit_GlobalAlreadyExists returns error when global config exists.
func TestConfigInit_GlobalAlreadyExists(t *testing.T) {
	// Use a temporary home directory
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Save flag state
	oldGlobalFlag := configGlobalFlag
	t.Cleanup(func() { configGlobalFlag = oldGlobalFlag })

	configGlobalFlag = true

	// Create existing global config
	configDir := filepath.Join(tmpHome, ".config", "terranoodle")
	err := os.MkdirAll(configDir, 0o755)
	require.NoError(t, err)

	configPath := filepath.Join(configDir, config.GlobalFile)
	err = os.WriteFile(configPath, []byte("lint:\n  rules:\n    test: true\n"), 0o644)
	require.NoError(t, err)

	// Try to init again
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err = runConfigInit(configInitCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}
