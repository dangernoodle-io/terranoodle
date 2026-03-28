package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMatchProfile_NoProfiles - empty GlobalConfig, returns "".
func TestMatchProfile_NoProfiles(t *testing.T) {
	cfg := &GlobalConfig{}
	result := MatchProfile(cfg, "/some/path")
	assert.Equal(t, "", result)
}

// TestMatchProfile_BindPrefix - profile with prefix bind path.
func TestMatchProfile_BindPrefix(t *testing.T) {
	cfg := &GlobalConfig{
		Profiles: map[string]Profile{
			"infra": {
				Bind: []string{"/Users/jae/Projects/infra"},
			},
		},
	}
	result := MatchProfile(cfg, "/Users/jae/Projects/infra/acme")
	assert.Equal(t, "infra", result)
}

// TestMatchProfile_BindExactMatch - cwd equals bind path exactly.
func TestMatchProfile_BindExactMatch(t *testing.T) {
	cfg := &GlobalConfig{
		Profiles: map[string]Profile{
			"infra": {
				Bind: []string{"/Users/jae/Projects/infra"},
			},
		},
	}
	result := MatchProfile(cfg, "/Users/jae/Projects/infra")
	assert.Equal(t, "infra", result)
}

// TestMatchProfile_BindNoMatch - cwd doesn't match any bind.
func TestMatchProfile_BindNoMatch(t *testing.T) {
	cfg := &GlobalConfig{
		Profiles: map[string]Profile{
			"infra": {
				Bind: []string{"/Users/jae/Projects/infra"},
			},
		},
	}
	result := MatchProfile(cfg, "/Users/jae/Projects/other")
	assert.Equal(t, "", result)
}

// TestMatchProfile_BindGlob - profile with glob bind path.
func TestMatchProfile_BindGlob(t *testing.T) {
	cfg := &GlobalConfig{
		Profiles: map[string]Profile{
			"modules": {
				Bind: []string{"/projects/*/modules"},
			},
		},
	}
	result := MatchProfile(cfg, "/projects/foo/modules")
	assert.Equal(t, "modules", result)
}

// TestMatchProfile_FirstMatchWins - two profiles both match, first alphabetically wins.
func TestMatchProfile_FirstMatchWins(t *testing.T) {
	cfg := &GlobalConfig{
		Profiles: map[string]Profile{
			"alpha": {
				Bind: []string{"/projects"},
			},
			"beta": {
				Bind: []string{"/projects"},
			},
		},
	}
	result := MatchProfile(cfg, "/projects/foo")
	assert.Equal(t, "alpha", result)
}

// TestMatchProfile_SkipsDefault - default profile has bind paths, they're ignored.
func TestMatchProfile_SkipsDefault(t *testing.T) {
	cfg := &GlobalConfig{
		Profiles: map[string]Profile{
			"default": {
				Bind: []string{"/should-be-ignored"},
			},
			"strict": {
				Bind: []string{"/projects/infra"},
			},
		},
	}
	result := MatchProfile(cfg, "/should-be-ignored")
	assert.Equal(t, "", result)

	result = MatchProfile(cfg, "/projects/infra")
	assert.Equal(t, "strict", result)
}

// TestDefaultGlobal - verify DefaultGlobal() has profiles.default with expected rules.
func TestDefaultGlobal(t *testing.T) {
	cfg := DefaultGlobal()
	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.Profiles)
	assert.NotNil(t, cfg.Profiles["default"])

	// Verify default profile has expected rules
	defaultProfile := cfg.Profiles["default"]
	assert.Equal(t, true, defaultProfile.Lint.Rules["missing-required"].Enabled)
	assert.Equal(t, true, defaultProfile.Lint.Rules["extra-inputs"].Enabled)
	assert.Equal(t, false, defaultProfile.Lint.Rules["missing-description"].Enabled)
}

// TestLoadGlobal_SaveGlobal_Roundtrip - save and load a GlobalConfig.
func TestLoadGlobal_SaveGlobal_Roundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.yml")

	original := &GlobalConfig{
		Lint: LintConfig{
			Rules: map[string]RuleConfig{
				"legacy-rule": {Enabled: true},
			},
		},
		Profiles: map[string]Profile{
			"default": {
				Lint: LintConfig{
					Rules: map[string]RuleConfig{
						"missing-required": {Enabled: true},
					},
				},
			},
			"strict": {
				Bind: []string{"/projects/infra"},
				Lint: LintConfig{
					Rules: map[string]RuleConfig{
						"missing-description": {Enabled: true},
					},
				},
			},
		},
	}

	err := SaveGlobal(filePath, original)
	require.NoError(t, err)

	loaded, err := LoadGlobal(filePath)
	require.NoError(t, err)

	// Verify legacy lint was preserved
	assert.Equal(t, true, loaded.Lint.Rules["legacy-rule"].Enabled)

	// Verify profiles exist
	assert.NotNil(t, loaded.Profiles["default"])
	assert.NotNil(t, loaded.Profiles["strict"])

	// Verify default profile rules
	assert.Equal(t, true, loaded.Profiles["default"].Lint.Rules["missing-required"].Enabled)

	// Verify strict profile bind and rules
	assert.Equal(t, []string{"/projects/infra"}, loaded.Profiles["strict"].Bind)
	assert.Equal(t, true, loaded.Profiles["strict"].Lint.Rules["missing-description"].Enabled)
}

// TestLoadGlobal_FileNotFound - nonexistent file returns error.
func TestLoadGlobal_FileNotFound(t *testing.T) {
	_, err := LoadGlobal("/nonexistent/path/config.yml")
	assert.Error(t, err)
}

// TestLoadGlobal_InvalidYAML - malformed YAML returns error.
func TestLoadGlobal_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.yml")

	content := `profiles:
  default:
    lint: [invalid yaml`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))

	_, err := LoadGlobal(filePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

// TestDiscover_ProfileActivated - global config with profile binding to a dir.
func TestDiscover_ProfileActivated(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up HOME to control GlobalPath
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "terranoodle")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	// Create project directory first to get its real path
	projectDir := filepath.Join(tmpDir, "test-project")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	// Write global config with strict profile bound to the actual project dir
	globalConfigPath := filepath.Join(configDir, "config.yml")
	globalContent := `profiles:
  default:
    lint:
      rules:
        missing-required: true
  strict:
    bind:
      - ` + projectDir + `
    lint:
      rules:
        missing-description: true
`
	require.NoError(t, os.WriteFile(globalConfigPath, []byte(globalContent), 0o644))

	projectConfig := filepath.Join(projectDir, ".terranoodle.yml")
	projectContent := `lint:
  rules:
    extra-inputs: false
`
	require.NoError(t, os.WriteFile(projectConfig, []byte(projectContent), 0o644))

	// Discover from project dir
	cfg, err := Discover(projectDir)
	require.NoError(t, err)

	// Verify merge: default <- strict profile <- project
	// Default has missing-required=true
	assert.Equal(t, true, cfg.Lint.Rules["missing-required"].Enabled)

	// Strict profile adds missing-description=true
	assert.Equal(t, true, cfg.Lint.Rules["missing-description"].Enabled)

	// Project sets extra-inputs=false
	assert.Equal(t, false, cfg.Lint.Rules["extra-inputs"].Enabled)
}

// TestDiscover_DefaultProfileOnly - global config with only default profile.
func TestDiscover_DefaultProfileOnly(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "terranoodle")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	// Write global config with only default profile
	globalConfigPath := filepath.Join(configDir, "config.yml")
	globalContent := `profiles:
  default:
    lint:
      rules:
        missing-required: false
`
	require.NoError(t, os.WriteFile(globalConfigPath, []byte(globalContent), 0o644))

	// Create project directory
	projectDir := filepath.Join(tmpDir, "test-project")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	// Discover without project config
	cfg, err := Discover(projectDir)
	require.NoError(t, err)

	// Verify default profile rules are applied
	assert.Equal(t, false, cfg.Lint.Rules["missing-required"].Enabled)
}

// TestDiscover_LegacyGlobalCompat - global config with flat lint (no profiles).
func TestDiscover_LegacyGlobalCompat(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "terranoodle")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	// Write legacy global config (flat lint, no profiles)
	globalConfigPath := filepath.Join(configDir, "config.yml")
	globalContent := `lint:
  rules:
    missing-required: false
    extra-inputs: true
`
	require.NoError(t, os.WriteFile(globalConfigPath, []byte(globalContent), 0o644))

	// Create project directory
	projectDir := filepath.Join(tmpDir, "test-project")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	// Discover without project config
	cfg, err := Discover(projectDir)
	require.NoError(t, err)

	// Verify legacy flat lint is applied
	assert.Equal(t, false, cfg.Lint.Rules["missing-required"].Enabled)
	assert.Equal(t, true, cfg.Lint.Rules["extra-inputs"].Enabled)
}

// TestDiscover_GlobalConfigError - global config load error is silently ignored.
func TestDiscover_GlobalConfigError(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "terranoodle")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	// Write invalid global config
	globalConfigPath := filepath.Join(configDir, "config.yml")
	globalContent := `lint:
  rules: [invalid yaml`
	require.NoError(t, os.WriteFile(globalConfigPath, []byte(globalContent), 0o644))

	// Create project directory
	projectDir := filepath.Join(tmpDir, "test-project")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	// Discover should succeed (error is silently ignored)
	cfg, err := Discover(projectDir)
	require.NoError(t, err)

	// Should have default rules (from Default())
	assert.NotNil(t, cfg)
	assert.Equal(t, true, cfg.Lint.Rules["missing-required"].Enabled)
}

// TestMatchBind_PrefixWithoutGlob - basic prefix matching.
func TestMatchBind_PrefixWithoutGlob(t *testing.T) {
	tests := []struct {
		name string
		cwd  string
		bind string
		want bool
	}{
		{
			name: "exact match",
			cwd:  "/projects/infra",
			bind: "/projects/infra",
			want: true,
		},
		{
			name: "subdirectory",
			cwd:  "/projects/infra/acme",
			bind: "/projects/infra",
			want: true,
		},
		{
			name: "partial match (not prefix)",
			cwd:  "/projects/infrastructure",
			bind: "/projects/infra",
			want: false,
		},
		{
			name: "no match",
			cwd:  "/other/path",
			bind: "/projects/infra",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchBind(tt.cwd, tt.bind)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestMatchBind_GlobPattern - glob-based matching.
func TestMatchBind_GlobPattern(t *testing.T) {
	tests := []struct {
		name string
		cwd  string
		bind string
		want bool
	}{
		{
			name: "single star",
			cwd:  "/projects/foo/modules",
			bind: "/projects/*/modules",
			want: true,
		},
		{
			name: "single star no match",
			cwd:  "/projects/foo/bar/modules",
			bind: "/projects/*/modules",
			want: false,
		},
		{
			name: "question mark",
			cwd:  "/projects/a/modules",
			bind: "/projects/?/modules",
			want: true,
		},
		{
			name: "bracket",
			cwd:  "/projects/a/modules",
			bind: "/projects/[a-z]/modules",
			want: true,
		},
		{
			name: "bracket no match",
			cwd:  "/projects/1/modules",
			bind: "/projects/[a-z]/modules",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchBind(tt.cwd, tt.bind)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestContainsGlob - detect glob characters.
func TestContainsGlob(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{
			name: "star",
			s:    "/projects/*/modules",
			want: true,
		},
		{
			name: "question",
			s:    "/projects/?/modules",
			want: true,
		},
		{
			name: "bracket",
			s:    "/projects/[a-z]/modules",
			want: true,
		},
		{
			name: "no glob",
			s:    "/projects/infra/modules",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsGlob(tt.s)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestValidateScaffoldProviders_NoDuplicates - distinct providers across profiles: no error.
func TestValidateScaffoldProviders_NoDuplicates(t *testing.T) {
	cfg := &GlobalConfig{
		Profiles: map[string]Profile{
			"aws": {
				Scaffold: ScaffoldConfig{
					Providers: []string{"aws", "random"},
				},
			},
			"azure": {
				Scaffold: ScaffoldConfig{
					Providers: []string{"azurerm"},
				},
			},
		},
	}

	err := ValidateScaffoldProviders(cfg)
	assert.NoError(t, err)
}

// TestValidateScaffoldProviders_Duplicate - two profiles with same provider: error contains
// provider name and both profile names.
func TestValidateScaffoldProviders_Duplicate(t *testing.T) {
	cfg := &GlobalConfig{
		Profiles: map[string]Profile{
			"profile-a": {
				Scaffold: ScaffoldConfig{
					Providers: []string{"aws", "random"},
				},
			},
			"profile-b": {
				Scaffold: ScaffoldConfig{
					Providers: []string{"aws", "azurerm"},
				},
			},
		},
	}

	err := ValidateScaffoldProviders(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aws")
	assert.Contains(t, err.Error(), "profile-a")
	assert.Contains(t, err.Error(), "profile-b")
}

// TestValidateScaffoldProviders_EmptyProviders - empty provider lists: no error.
func TestValidateScaffoldProviders_EmptyProviders(t *testing.T) {
	cfg := &GlobalConfig{
		Profiles: map[string]Profile{
			"empty-a": {
				Scaffold: ScaffoldConfig{
					Providers: []string{},
				},
			},
			"empty-b": {
				Scaffold: ScaffoldConfig{
					Providers: []string{},
				},
			},
		},
	}

	err := ValidateScaffoldProviders(cfg)
	assert.NoError(t, err)
}

// TestScaffoldProfileForProvider_Match - provider in named profile returns that profile name.
func TestScaffoldProfileForProvider_Match(t *testing.T) {
	cfg := &GlobalConfig{
		Profiles: map[string]Profile{
			"aws-config": {
				Scaffold: ScaffoldConfig{
					Providers: []string{"aws", "random"},
				},
			},
			"azure-config": {
				Scaffold: ScaffoldConfig{
					Providers: []string{"azurerm"},
				},
			},
		},
	}

	result := ScaffoldProfileForProvider(cfg, "aws")
	assert.Equal(t, "aws-config", result)

	result = ScaffoldProfileForProvider(cfg, "azurerm")
	assert.Equal(t, "azure-config", result)
}

// TestScaffoldProfileForProvider_NoMatch - unknown provider returns "".
func TestScaffoldProfileForProvider_NoMatch(t *testing.T) {
	cfg := &GlobalConfig{
		Profiles: map[string]Profile{
			"aws-config": {
				Scaffold: ScaffoldConfig{
					Providers: []string{"aws"},
				},
			},
		},
	}

	result := ScaffoldProfileForProvider(cfg, "gcp")
	assert.Equal(t, "", result)
}

// TestScaffoldProfileForProvider_NilConfig - nil GlobalConfig returns "".
func TestScaffoldProfileForProvider_NilConfig(t *testing.T) {
	result := ScaffoldProfileForProvider(nil, "aws")
	assert.Equal(t, "", result)
}

// TestLoadGlobal_SaveGlobal_ScaffoldRoundtrip - save GlobalConfig with scaffold config,
// load it back, verify scaffold fields preserved.
func TestLoadGlobal_SaveGlobal_ScaffoldRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.yml")

	original := &GlobalConfig{
		Profiles: map[string]Profile{
			"default": {
				Lint: LintConfig{
					Rules: map[string]RuleConfig{
						"missing-required": {Enabled: true},
					},
				},
			},
			"aws-prod": {
				Bind: []string{"/projects/acme-corp/aws"},
				Scaffold: ScaffoldConfig{
					State:     "s3://tk-test-000-state/prod",
					Providers: []string{"aws", "random"},
				},
				Lint: LintConfig{
					Rules: map[string]RuleConfig{
						"missing-description": {Enabled: true},
					},
				},
			},
		},
	}

	err := SaveGlobal(filePath, original)
	require.NoError(t, err)

	loaded, err := LoadGlobal(filePath)
	require.NoError(t, err)

	// Verify default profile
	assert.NotNil(t, loaded.Profiles["default"])
	assert.Equal(t, true, loaded.Profiles["default"].Lint.Rules["missing-required"].Enabled)

	// Verify aws-prod profile scaffold fields
	awsProd := loaded.Profiles["aws-prod"]
	assert.NotNil(t, awsProd)
	assert.Equal(t, "/projects/acme-corp/aws", awsProd.Bind[0])
	assert.Equal(t, "s3://tk-test-000-state/prod", awsProd.Scaffold.State)
	assert.Equal(t, []string{"aws", "random"}, awsProd.Scaffold.Providers)
}

// TestLoadGlobal_DuplicateScaffoldProviders - file with duplicate providers fails LoadGlobal.
func TestLoadGlobal_DuplicateScaffoldProviders(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.yml")

	content := `profiles:
  default:
    lint:
      rules:
        missing-required: true
  profile-a:
    scaffold:
      state: s3://bucket/a
      providers:
        - aws
  profile-b:
    scaffold:
      state: s3://bucket/b
      providers:
        - aws
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))

	_, err := LoadGlobal(filePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aws")
	assert.Contains(t, err.Error(), "profile-a")
	assert.Contains(t, err.Error(), "profile-b")
}
