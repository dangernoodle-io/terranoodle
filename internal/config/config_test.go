package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Test RuleConfig UnmarshalYAML - bool form.
func TestRuleConfigUnmarshalYAML_BoolForm(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    bool
		wantErr bool
	}{
		{
			name:    "true",
			yaml:    "true",
			want:    true,
			wantErr: false,
		},
		{
			name:    "false",
			yaml:    "false",
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rc RuleConfig
			err := yaml.Unmarshal([]byte(tt.yaml), &rc)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, rc.Enabled)
				assert.Nil(t, rc.Options)
			}
		})
	}
}

// Test RuleConfig UnmarshalYAML - object form.
func TestRuleConfigUnmarshalYAML_ObjectForm(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantEnabled bool
		wantOptions map[string]interface{}
		wantErr     bool
	}{
		{
			name:        "enabled only",
			yaml:        "enabled: true",
			wantEnabled: true,
			wantOptions: nil,
			wantErr:     false,
		},
		{
			name:        "enabled with option",
			yaml:        "enabled: true\nmax-warnings: 10",
			wantEnabled: true,
			wantOptions: map[string]interface{}{"max-warnings": 10},
			wantErr:     false,
		},
		{
			name:        "enabled false with option",
			yaml:        "enabled: false\nmax-warnings: 10",
			wantEnabled: false,
			wantOptions: map[string]interface{}{"max-warnings": 10},
			wantErr:     false,
		},
		{
			name:        "multiple options",
			yaml:        "enabled: true\nmax-warnings: 10\nmin-severity: low",
			wantEnabled: true,
			wantOptions: map[string]interface{}{"max-warnings": 10, "min-severity": "low"},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rc RuleConfig
			err := yaml.Unmarshal([]byte(tt.yaml), &rc)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantEnabled, rc.Enabled)
				assert.Equal(t, tt.wantOptions, rc.Options)
			}
		})
	}
}

// Test RuleConfig UnmarshalYAML - invalid form.
func TestRuleConfigUnmarshalYAML_InvalidForm(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name:    "list form",
			yaml:    "[true, false]",
			wantErr: true,
		},
		{
			name:    "null",
			yaml:    "null",
			wantErr: false, // null decodes to false
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rc RuleConfig
			err := yaml.Unmarshal([]byte(tt.yaml), &rc)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Test RuleConfig MarshalYAML - short form (no options).
func TestRuleConfigMarshalYAML_ShortForm(t *testing.T) {
	rc := RuleConfig{Enabled: true}
	data, err := yaml.Marshal(rc)
	require.NoError(t, err)

	// Should marshal to just "true"
	assert.Equal(t, "true\n", string(data))

	// Roundtrip
	var rc2 RuleConfig
	err = yaml.Unmarshal(data, &rc2)
	require.NoError(t, err)
	assert.Equal(t, rc.Enabled, rc2.Enabled)
	assert.Nil(t, rc2.Options)
}

// Test RuleConfig MarshalYAML - long form (with options).
func TestRuleConfigMarshalYAML_LongForm(t *testing.T) {
	rc := RuleConfig{
		Enabled: true,
		Options: map[string]interface{}{
			"max-warnings": 10,
			"min-severity": "low",
		},
	}

	data, err := yaml.Marshal(rc)
	require.NoError(t, err)

	// Should contain "enabled: true" and the options
	assert.Contains(t, string(data), "enabled: true")
	assert.Contains(t, string(data), "max-warnings: 10")
	assert.Contains(t, string(data), "min-severity: low")

	// Roundtrip
	var rc2 RuleConfig
	err = yaml.Unmarshal(data, &rc2)
	require.NoError(t, err)
	assert.Equal(t, rc.Enabled, rc2.Enabled)
	assert.Equal(t, rc.Options, rc2.Options)
}

// Test Load - valid file.
func TestLoad_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, ".terranoodle.yml")

	content := `lint:
  rules:
    missing-required: true
    extra-input: false
  exclude-dirs:
    - node_modules
    - vendor
`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))

	cfg, err := Load(filePath)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, true, cfg.Lint.Rules["missing-required"].Enabled)
	assert.Equal(t, false, cfg.Lint.Rules["extra-input"].Enabled)
	assert.Equal(t, []string{"node_modules", "vendor"}, cfg.Lint.ExcludeDirs)
}

// Test Load - nonexistent file.
func TestLoad_NonexistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/.terranoodle.yml")
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "read")
}

// Test Load - invalid YAML.
func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, ".terranoodle.yml")

	content := `lint:
  rules: [invalid yaml structure`
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))

	cfg, err := Load(filePath)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "parse")
}

// Test Save - writes valid YAML and creates parent dirs.
func TestSave_CreatesParentDirs(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "subdir1", "subdir2", ".terranoodle.yml")

	cfg := &Config{
		Lint: LintConfig{
			Rules: map[string]RuleConfig{
				"test-rule": {Enabled: true},
			},
		},
	}

	err := Save(filePath, cfg)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(filePath)
	require.NoError(t, err)

	// Verify content is valid YAML
	loaded, err := Load(filePath)
	require.NoError(t, err)
	assert.Equal(t, cfg.Lint.Rules["test-rule"].Enabled, loaded.Lint.Rules["test-rule"].Enabled)
}

// Test Save then Load roundtrip.
func TestSaveAndLoad_Roundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, ".terranoodle.yml")

	original := &Config{
		Lint: LintConfig{
			Rules: map[string]RuleConfig{
				"missing-required": {Enabled: true},
				"extra-input": {
					Enabled: false,
					Options: map[string]interface{}{
						"max-warnings": 10,
					},
				},
			},
			ExcludeDirs: []string{"node_modules", "vendor"},
			Overrides: []Override{
				{
					Paths: []string{"dev/*"},
					Rules: map[string]RuleConfig{
						"missing-required": {Enabled: false},
					},
				},
			},
		},
	}

	// Save
	err := Save(filePath, original)
	require.NoError(t, err)

	// Load
	loaded, err := Load(filePath)
	require.NoError(t, err)

	// Verify equality
	assert.Equal(t, original.Lint.Rules["missing-required"].Enabled, loaded.Lint.Rules["missing-required"].Enabled)
	assert.Equal(t, original.Lint.Rules["extra-input"].Enabled, loaded.Lint.Rules["extra-input"].Enabled)
	assert.Equal(t, original.Lint.ExcludeDirs, loaded.Lint.ExcludeDirs)
	assert.Equal(t, len(original.Lint.Overrides), len(loaded.Lint.Overrides))
}

// Test Discover - project config only.
func TestDiscover_ProjectConfigOnly(t *testing.T) {
	tmpDir := t.TempDir()

	// Create project config
	projectConfigPath := filepath.Join(tmpDir, ".terranoodle.yml")
	content := `lint:
  rules:
    missing-required: true
`
	require.NoError(t, os.WriteFile(projectConfigPath, []byte(content), 0o644))

	// Discover from that directory
	cfg, err := Discover(tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, true, cfg.Lint.Rules["missing-required"].Enabled)
}

// Test Discover - no config found.
func TestDiscover_NoConfigFound(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, err := Discover(tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, Default().Lint.Rules, cfg.Lint.Rules) // Should have default rules
}

// Test Discover - walk-up behavior.
func TestDiscover_WalkUp(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directories
	parentDir := tmpDir
	childDir := filepath.Join(parentDir, "child", "grandchild")
	require.NoError(t, os.MkdirAll(childDir, 0o755))

	// Create config in parent
	parentConfigPath := filepath.Join(parentDir, ".terranoodle.yml")
	content := `lint:
  rules:
    missing-required: true
    extra-input: false
`
	require.NoError(t, os.WriteFile(parentConfigPath, []byte(content), 0o644))

	// Discover from child directory
	cfg, err := Discover(childDir)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, true, cfg.Lint.Rules["missing-required"].Enabled)
	assert.Equal(t, false, cfg.Lint.Rules["extra-input"].Enabled)
}

// Test Merge - rules merged per-key (b wins).
func TestMerge_RulesMergedPerKey(t *testing.T) {
	a := &Config{
		Lint: LintConfig{
			Rules: map[string]RuleConfig{
				"rule-a": {Enabled: true},
				"rule-b": {Enabled: true},
				"rule-c": {Enabled: false},
			},
		},
	}

	b := &Config{
		Lint: LintConfig{
			Rules: map[string]RuleConfig{
				"rule-b": {Enabled: false},
				"rule-d": {Enabled: true},
			},
		},
	}

	result := Merge(a, b)

	// rule-a unchanged (only in a)
	assert.Equal(t, true, result.Lint.Rules["rule-a"].Enabled)
	// rule-b overridden by b
	assert.Equal(t, false, result.Lint.Rules["rule-b"].Enabled)
	// rule-c unchanged (only in a)
	assert.Equal(t, false, result.Lint.Rules["rule-c"].Enabled)
	// rule-d from b
	assert.Equal(t, true, result.Lint.Rules["rule-d"].Enabled)
}

// Test Merge - exclude-dirs replaced by b.
func TestMerge_ExcludeDirsReplaced(t *testing.T) {
	a := &Config{
		Lint: LintConfig{
			ExcludeDirs: []string{"node_modules", "vendor"},
		},
	}

	b := &Config{
		Lint: LintConfig{
			ExcludeDirs: []string{"build", "dist"},
		},
	}

	result := Merge(a, b)
	assert.Equal(t, []string{"build", "dist"}, result.Lint.ExcludeDirs)
}

// Test Merge - exclude-dirs b empty keeps a.
func TestMerge_ExcludeDirsEmpty(t *testing.T) {
	a := &Config{
		Lint: LintConfig{
			ExcludeDirs: []string{"node_modules"},
		},
	}

	b := &Config{
		Lint: LintConfig{
			ExcludeDirs: []string{},
		},
	}

	result := Merge(a, b)
	assert.Equal(t, []string{"node_modules"}, result.Lint.ExcludeDirs)
}

// Test Merge - overrides appended.
func TestMerge_OverridesAppended(t *testing.T) {
	a := &Config{
		Lint: LintConfig{
			Overrides: []Override{
				{Paths: []string{"a/*"}},
			},
		},
	}

	b := &Config{
		Lint: LintConfig{
			Overrides: []Override{
				{Paths: []string{"b/*"}},
			},
		},
	}

	result := Merge(a, b)
	assert.Equal(t, 2, len(result.Lint.Overrides))
	assert.Equal(t, []string{"a/*"}, result.Lint.Overrides[0].Paths)
	assert.Equal(t, []string{"b/*"}, result.Lint.Overrides[1].Paths)
}

// Test RulesForPath - base rules only.
func TestRulesForPath_BaseRulesOnly(t *testing.T) {
	cfg := &LintConfig{
		Rules: map[string]RuleConfig{
			"rule-a": {Enabled: true},
			"rule-b": {Enabled: false},
		},
	}

	rules := cfg.RulesForPath("some/file.tf")
	assert.Equal(t, true, rules["rule-a"])
	assert.Equal(t, false, rules["rule-b"])
}

// Test RulesForPath - with one override match.
func TestRulesForPath_OneOverrideMatch(t *testing.T) {
	cfg := &LintConfig{
		Rules: map[string]RuleConfig{
			"rule-a": {Enabled: true},
		},
		Overrides: []Override{
			{
				Paths: []string{"dev/*"},
				Rules: map[string]RuleConfig{
					"rule-a": {Enabled: false},
				},
			},
		},
	}

	// Match
	rules := cfg.RulesForPath("dev/test.tf")
	assert.Equal(t, false, rules["rule-a"])

	// No match
	rules = cfg.RulesForPath("prod/test.tf")
	assert.Equal(t, true, rules["rule-a"])
}

// Test RulesForPath - with two overrides (last wins).
func TestRulesForPath_TwoOverridesLastWins(t *testing.T) {
	cfg := &LintConfig{
		Rules: map[string]RuleConfig{
			"rule-a": {Enabled: true},
		},
		Overrides: []Override{
			{
				Paths: []string{"*.tf"},
				Rules: map[string]RuleConfig{
					"rule-a": {Enabled: false},
				},
			},
			{
				Paths: []string{"prod-*.tf"},
				Rules: map[string]RuleConfig{
					"rule-a": {Enabled: true},
				},
			},
		},
	}

	// Matches both (prod-file.tf), second wins
	rules := cfg.RulesForPath("prod-file.tf")
	assert.Equal(t, true, rules["rule-a"])

	// Matches first only
	rules = cfg.RulesForPath("dev-file.tf")
	assert.Equal(t, false, rules["rule-a"])
}

// Test IsRuleEnabled - configured rule.
func TestIsRuleEnabled_ConfiguredRule(t *testing.T) {
	cfg := &LintConfig{
		Rules: map[string]RuleConfig{
			"test-rule": {Enabled: true},
		},
	}

	assert.Equal(t, true, cfg.IsRuleEnabled("test-rule", "any/path.tf"))
}

// Test IsRuleEnabled - unknown rule defaults to true.
func TestIsRuleEnabled_UnknownRuleDefaultsTrue(t *testing.T) {
	cfg := &LintConfig{
		Rules: map[string]RuleConfig{},
	}

	assert.Equal(t, true, cfg.IsRuleEnabled("unknown-rule", "any/path.tf"))
}

// Test Get - existing rule.
func TestGet_ExistingRule(t *testing.T) {
	cfg := &Config{
		Lint: LintConfig{
			Rules: map[string]RuleConfig{
				"test-rule": {Enabled: true},
			},
		},
	}

	val, err := cfg.Get("lint.rules.test-rule")
	require.NoError(t, err)
	assert.Equal(t, "true", val)
}

// Test Get - nonexistent rule.
func TestGet_NonexistentRule(t *testing.T) {
	cfg := &Config{
		Lint: LintConfig{
			Rules: map[string]RuleConfig{},
		},
	}

	_, err := cfg.Get("lint.rules.nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

// Test Get - exclude-dirs.
func TestGet_ExcludeDirs(t *testing.T) {
	cfg := &Config{
		Lint: LintConfig{
			ExcludeDirs: []string{"node_modules", "vendor"},
		},
	}

	val, err := cfg.Get("lint.exclude-dirs")
	require.NoError(t, err)
	assert.Equal(t, "node_modules,vendor", val)
}

// Test Get - exclude-dirs empty.
func TestGet_ExcludeDirsEmpty(t *testing.T) {
	cfg := &Config{
		Lint: LintConfig{
			ExcludeDirs: []string{},
		},
	}

	val, err := cfg.Get("lint.exclude-dirs")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

// Test Get - unknown key.
func TestGet_UnknownKey(t *testing.T) {
	cfg := &Config{}

	_, err := cfg.Get("unknown.key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown key")
}

// Test Set - set rule to true.
func TestSet_RuleTrue(t *testing.T) {
	cfg := &Config{}

	err := cfg.Set("lint.rules.test-rule", "true")
	require.NoError(t, err)
	assert.Equal(t, true, cfg.Lint.Rules["test-rule"].Enabled)
}

// Test Set - set rule to false.
func TestSet_RuleFalse(t *testing.T) {
	cfg := &Config{}

	err := cfg.Set("lint.rules.test-rule", "false")
	require.NoError(t, err)
	assert.Equal(t, false, cfg.Lint.Rules["test-rule"].Enabled)
}

// Test Set - set exclude-dirs.
func TestSet_ExcludeDirs(t *testing.T) {
	cfg := &Config{}

	err := cfg.Set("lint.exclude-dirs", "node_modules,vendor")
	require.NoError(t, err)
	assert.Equal(t, []string{"node_modules", "vendor"}, cfg.Lint.ExcludeDirs)
}

// Test Set - clear exclude-dirs.
func TestSet_ExcludeDirsClear(t *testing.T) {
	cfg := &Config{
		Lint: LintConfig{
			ExcludeDirs: []string{"node_modules"},
		},
	}

	err := cfg.Set("lint.exclude-dirs", "")
	require.NoError(t, err)
	assert.Nil(t, cfg.Lint.ExcludeDirs)
}

// Test Set - invalid rule value.
func TestSet_InvalidRuleValue(t *testing.T) {
	cfg := &Config{}

	err := cfg.Set("lint.rules.test-rule", "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be true or false")
}

// Test Set - unknown key.
func TestSet_UnknownKey(t *testing.T) {
	cfg := &Config{}

	err := cfg.Set("unknown.key", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown key")
}

// Test Default - returns all built-in rules enabled.
func TestDefault(t *testing.T) {
	cfg := Default()

	assert.NotNil(t, cfg)
	assert.Equal(t, 12, len(cfg.Lint.Rules))
	assert.Equal(t, true, cfg.Lint.Rules["missing-required"].Enabled)
	assert.Equal(t, true, cfg.Lint.Rules["extra-input"].Enabled)
	assert.Equal(t, true, cfg.Lint.Rules["type-mismatch"].Enabled)
	assert.Equal(t, true, cfg.Lint.Rules["source-ref-semver"].Enabled)
	assert.Equal(t, false, cfg.Lint.Rules["source-protocol"].Enabled)
	assert.Equal(t, false, cfg.Lint.Rules["missing-description"].Enabled)
	assert.Equal(t, false, cfg.Lint.Rules["non-snake-case"].Enabled)
	assert.Equal(t, false, cfg.Lint.Rules["unused-variable"].Enabled)
	assert.Equal(t, false, cfg.Lint.Rules["optional-without-default"].Enabled)
	assert.Equal(t, false, cfg.Lint.Rules["missing-include-expose"].Enabled)
	assert.Equal(t, false, cfg.Lint.Rules["allowed-filenames"].Enabled)
	assert.Equal(t, false, cfg.Lint.Rules["versions-tf"].Enabled)
}
