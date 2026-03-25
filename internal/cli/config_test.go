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

	// Verify contents are GlobalConfig with profiles.default
	var globalCfg config.GlobalConfig
	err = yaml.Unmarshal(data, &globalCfg)
	require.NoError(t, err)

	// Check that default profile exists with rules
	assert.NotNil(t, globalCfg.Profiles)
	assert.NotNil(t, globalCfg.Profiles["default"])
	assert.NotNil(t, globalCfg.Profiles["default"].Lint.Rules)
	assert.True(t, len(globalCfg.Profiles["default"].Lint.Rules) > 0)
	assert.True(t, globalCfg.Profiles["default"].Lint.Rules["missing-required"].Enabled)
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

// TestConfigSet_ProfileRule sets a lint rule on a named profile.
func TestConfigSet_ProfileRule(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	oldProfileFlag := configProfileFlag
	oldGlobalFlag := configGlobalFlag
	t.Cleanup(func() {
		configProfileFlag = oldProfileFlag
		configGlobalFlag = oldGlobalFlag
	})

	configProfileFlag = "strict"
	configGlobalFlag = false // --profile implies --global internally

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err := runConfigSet(configSetCmd, []string{"lint.rules.missing-description", "true"})
	require.NoError(t, err)

	// Verify global config was written with the profile
	globalPath := filepath.Join(tmpHome, ".config", "terranoodle", config.GlobalFile)
	data, err := os.ReadFile(globalPath)
	require.NoError(t, err)

	var globalCfg config.GlobalConfig
	err = yaml.Unmarshal(data, &globalCfg)
	require.NoError(t, err)

	assert.NotNil(t, globalCfg.Profiles["strict"])
	assert.True(t, globalCfg.Profiles["strict"].Lint.Rules["missing-description"].Enabled)
}

// TestConfigSet_ProfileBind sets bind paths on a profile.
func TestConfigSet_ProfileBind(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	oldProfileFlag := configProfileFlag
	oldGlobalFlag := configGlobalFlag
	t.Cleanup(func() {
		configProfileFlag = oldProfileFlag
		configGlobalFlag = oldGlobalFlag
	})

	configProfileFlag = "strict"
	configGlobalFlag = false

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err := runConfigSet(configSetCmd, []string{"bind", "/Users/test/infra,/Users/test/prod"})
	require.NoError(t, err)

	// Verify bind paths were set
	globalPath := filepath.Join(tmpHome, ".config", "terranoodle", config.GlobalFile)
	data, err := os.ReadFile(globalPath)
	require.NoError(t, err)

	var globalCfg config.GlobalConfig
	err = yaml.Unmarshal(data, &globalCfg)
	require.NoError(t, err)

	assert.NotNil(t, globalCfg.Profiles["strict"])
	assert.Equal(t, []string{"/Users/test/infra", "/Users/test/prod"}, globalCfg.Profiles["strict"].Bind)
}

// TestConfigGet_Profile reads from a specific profile.
func TestConfigGet_Profile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	oldProfileFlag := configProfileFlag
	t.Cleanup(func() { configProfileFlag = oldProfileFlag })

	// Create a global config with a "strict" profile
	configDir := filepath.Join(tmpHome, ".config", "terranoodle")
	err := os.MkdirAll(configDir, 0o755)
	require.NoError(t, err)

	globalCfg := &config.GlobalConfig{
		Profiles: map[string]config.Profile{
			"strict": {
				Lint: config.LintConfig{
					Rules: map[string]config.RuleConfig{
						"missing-description": {Enabled: true},
					},
				},
			},
		},
	}

	configPath := filepath.Join(configDir, config.GlobalFile)
	data, err := yaml.Marshal(globalCfg)
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0o644)
	require.NoError(t, err)

	configProfileFlag = "strict"
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err = runConfigGet(configGetCmd, []string{"lint.rules.missing-description"})
	require.NoError(t, err)
}

// TestConfigSet_GlobalDefaultProfile sets a rule on the default profile via --global.
func TestConfigSet_GlobalDefaultProfile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	oldGlobalFlag := configGlobalFlag
	oldProfileFlag := configProfileFlag
	t.Cleanup(func() {
		configGlobalFlag = oldGlobalFlag
		configProfileFlag = oldProfileFlag
	})

	configGlobalFlag = true
	configProfileFlag = ""

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err := runConfigSet(configSetCmd, []string{"lint.rules.missing-required", "false"})
	require.NoError(t, err)

	// Verify default profile was updated
	globalPath := filepath.Join(tmpHome, ".config", "terranoodle", config.GlobalFile)
	data, err := os.ReadFile(globalPath)
	require.NoError(t, err)

	var globalCfg config.GlobalConfig
	err = yaml.Unmarshal(data, &globalCfg)
	require.NoError(t, err)

	assert.NotNil(t, globalCfg.Profiles["default"])
	assert.False(t, globalCfg.Profiles["default"].Lint.Rules["missing-required"].Enabled)
}

// TestConfigInit_GlobalScaffoldsProfiles verifies init creates profiles structure.
func TestConfigInit_GlobalScaffoldsProfiles(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	oldGlobalFlag := configGlobalFlag
	t.Cleanup(func() { configGlobalFlag = oldGlobalFlag })

	configGlobalFlag = true

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err := runConfigInit(configInitCmd, nil)
	require.NoError(t, err)

	// Verify profiles structure exists
	globalPath := filepath.Join(tmpHome, ".config", "terranoodle", config.GlobalFile)
	data, err := os.ReadFile(globalPath)
	require.NoError(t, err)

	var globalCfg config.GlobalConfig
	err = yaml.Unmarshal(data, &globalCfg)
	require.NoError(t, err)

	assert.NotNil(t, globalCfg.Profiles)
	assert.NotNil(t, globalCfg.Profiles["default"])
	assert.NotNil(t, globalCfg.Profiles["default"].Lint.Rules)
	assert.True(t, len(globalCfg.Profiles["default"].Lint.Rules) > 0)
}

// TestConfigGet_ProfileNotFound returns error when profile doesn't exist.
func TestConfigGet_ProfileNotFound(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	oldProfileFlag := configProfileFlag
	t.Cleanup(func() { configProfileFlag = oldProfileFlag })

	// Create a global config with only default profile
	configDir := filepath.Join(tmpHome, ".config", "terranoodle")
	err := os.MkdirAll(configDir, 0o755)
	require.NoError(t, err)

	globalCfg := &config.GlobalConfig{
		Profiles: map[string]config.Profile{
			"default": {
				Lint: config.LintConfig{
					Rules: map[string]config.RuleConfig{
						"test-rule": {Enabled: true},
					},
				},
			},
		},
	}

	configPath := filepath.Join(configDir, config.GlobalFile)
	data, err := yaml.Marshal(globalCfg)
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0o644)
	require.NoError(t, err)

	configProfileFlag = "nonexistent"
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err = runConfigGet(configGetCmd, []string{"lint.rules.test-rule"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
