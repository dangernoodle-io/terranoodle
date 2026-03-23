package prompt

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"dangernoodle.io/terra-tools/internal/state/config"
)

// TestManualID_EnteredValueWithSave tests the ManualID function with a valid
// ID and request to save as template.
func TestManualID_EnteredValueWithSave(t *testing.T) {
	input := "acme-project-123\ny\n"
	r := strings.NewReader(input)
	w := &bytes.Buffer{}

	id, save, err := ManualID(r, w, "aws_s3_bucket.main", "aws_s3_bucket", nil)

	require.NoError(t, err)
	assert.Equal(t, "acme-project-123", id)
	assert.True(t, save)
}

// TestManualID_Skip tests skipping the prompt with "skip" keyword.
func TestManualID_Skip(t *testing.T) {
	input := "skip\n"
	r := strings.NewReader(input)
	w := &bytes.Buffer{}

	id, save, err := ManualID(r, w, "aws_s3_bucket.main", "aws_s3_bucket", nil)

	require.NoError(t, err)
	assert.Equal(t, "", id)
	assert.False(t, save)
}

// TestManualID_EmptyLine tests empty line treated as skip.
func TestManualID_EmptyLine(t *testing.T) {
	input := "\n"
	r := strings.NewReader(input)
	w := &bytes.Buffer{}

	id, save, err := ManualID(r, w, "aws_s3_bucket.main", "aws_s3_bucket", nil)

	require.NoError(t, err)
	assert.Equal(t, "", id)
	assert.False(t, save)
}

// TestManualID_EOFAfterID tests EOF after ID (no answer to save prompt).
func TestManualID_EOFAfterID(t *testing.T) {
	input := "acme-project-123\n"
	r := strings.NewReader(input)
	w := &bytes.Buffer{}

	id, save, err := ManualID(r, w, "aws_s3_bucket.main", "aws_s3_bucket", nil)

	require.NoError(t, err)
	assert.Equal(t, "acme-project-123", id)
	assert.False(t, save)
}

// TestManualID_WithFields tests output of available fields.
func TestManualID_WithFields(t *testing.T) {
	fields := map[string]string{
		"project": "acme-project",
		"name":    "acme-instance",
	}
	input := "acme-project-123\nn\n"
	r := strings.NewReader(input)
	w := &bytes.Buffer{}

	id, save, err := ManualID(r, w, "aws_s3_bucket.main", "aws_s3_bucket", fields)

	require.NoError(t, err)
	assert.Equal(t, "acme-project-123", id)
	assert.False(t, save)

	output := w.String()
	assert.Contains(t, output, "Available fields:")
	assert.Contains(t, output, ".project = \"acme-project\"")
	assert.Contains(t, output, ".name = \"acme-instance\"")
}

// TestManualID_SaveNo tests explicit "no" to save.
func TestManualID_SaveNo(t *testing.T) {
	input := "acme-project-123\nn\n"
	r := strings.NewReader(input)
	w := &bytes.Buffer{}

	id, save, err := ManualID(r, w, "aws_s3_bucket.main", "aws_s3_bucket", nil)

	require.NoError(t, err)
	assert.Equal(t, "acme-project-123", id)
	assert.False(t, save)
}

// TestManualID_SaveYesUppercase tests uppercase "Y" as yes to save.
func TestManualID_SaveYesUppercase(t *testing.T) {
	input := "acme-project-123\nY\n"
	r := strings.NewReader(input)
	w := &bytes.Buffer{}

	id, save, err := ManualID(r, w, "aws_s3_bucket.main", "aws_s3_bucket", nil)

	require.NoError(t, err)
	assert.Equal(t, "acme-project-123", id)
	assert.True(t, save)
}

// TestSaveTypeMapping_NewKey tests saving a new type mapping.
func TestSaveTypeMapping_NewKey(t *testing.T) {
	configPath := t.TempDir() + "/config.yaml"

	// Write minimal YAML config
	minimalConfig := "types: {}\n"
	err := writeFile(configPath, minimalConfig)
	require.NoError(t, err)

	// Save a type mapping
	err = SaveTypeMapping(configPath, "aws_s3_bucket", "{{ .name }}")
	require.NoError(t, err)

	// Read and verify
	data, err := readFile(configPath)
	require.NoError(t, err)

	var cfg config.Config
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.NotNil(t, cfg.Types)
	assert.Equal(t, "{{ .name }}", cfg.Types["aws_s3_bucket"].ID)
}

// TestSaveTypeMapping_OverwriteExisting tests overwriting an existing type mapping.
func TestSaveTypeMapping_OverwriteExisting(t *testing.T) {
	configPath := t.TempDir() + "/config.yaml"

	// Write config with existing type mapping
	existingConfig := `types:
  aws_s3_bucket:
    id: "old-template"
`
	err := writeFile(configPath, existingConfig)
	require.NoError(t, err)

	// Overwrite the type mapping
	err = SaveTypeMapping(configPath, "aws_s3_bucket", "{{ .new }}")
	require.NoError(t, err)

	// Read and verify
	data, err := readFile(configPath)
	require.NoError(t, err)

	var cfg config.Config
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	assert.Equal(t, "{{ .new }}", cfg.Types["aws_s3_bucket"].ID)
}

// TestSaveResolverAndType_FullRoundtrip tests saving both resolver and type.
func TestSaveResolverAndType_FullRoundtrip(t *testing.T) {
	configPath := t.TempDir() + "/config.yaml"

	// Write minimal YAML config
	minimalConfig := "types: {}\nresolvers: {}\n"
	err := writeFile(configPath, minimalConfig)
	require.NoError(t, err)

	// Create resolver result
	result := &ResolverResult{
		Name: "badge_id",
		Get:  "/api/badges",
		Pick: "id",
		Use:  nil,
		TypeMapping: config.TypeMapping{
			Use: []string{"badge_id"},
			ID:  "$badge_id",
		},
	}

	// Save resolver and type
	err = SaveResolverAndType(configPath, "google_project_badge", result)
	require.NoError(t, err)

	// Read and verify
	data, err := readFile(configPath)
	require.NoError(t, err)

	var cfg config.Config
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	// Verify resolver was saved
	assert.NotNil(t, cfg.Resolvers)
	assert.NotNil(t, cfg.Resolvers["badge_id"])
	assert.Equal(t, "/api/badges", cfg.Resolvers["badge_id"].Get)
	assert.Equal(t, "id", cfg.Resolvers["badge_id"].Pick)

	// Verify type mapping was saved
	assert.NotNil(t, cfg.Types)
	assert.NotNil(t, cfg.Types["google_project_badge"])
	assert.Equal(t, "$badge_id", cfg.Types["google_project_badge"].ID)
	assert.Equal(t, []string{"badge_id"}, cfg.Types["google_project_badge"].Use)
}

// TestRenderSimple_FieldSubstitution tests field template substitution.
func TestRenderSimple_FieldSubstitution(t *testing.T) {
	fields := map[string]string{
		"project": "acme-project",
		"name":    "acme-instance",
	}
	tmpl := "/projects/.project/instances/.name"
	result := renderSimple(tmpl, fields, nil)
	assert.Equal(t, "/projects/acme-project/instances/acme-instance", result)
}

// TestRenderSimple_VarSubstitution tests var template substitution.
func TestRenderSimple_VarSubstitution(t *testing.T) {
	vars := map[string]string{
		"org":     "acme-corp",
		"project": "alpha",
	}
	tmpl := "/orgs/$org/projects/$project"
	result := renderSimple(tmpl, nil, vars)
	assert.Equal(t, "/orgs/acme-corp/projects/alpha", result)
}

// TestRenderSimple_LongestKeyFirst tests longest key first substitution.
func TestRenderSimple_LongestKeyFirst(t *testing.T) {
	fields := map[string]string{
		"project":    "p",
		"project_id": "q",
	}
	tmpl := "/api/.project_id"
	result := renderSimple(tmpl, fields, nil)
	assert.Equal(t, "/api/q", result)
}

// TestRenderSimple_MixedFieldsAndVars tests mixed field and var substitution.
func TestRenderSimple_MixedFieldsAndVars(t *testing.T) {
	fields := map[string]string{
		"project": "acme-project",
	}
	vars := map[string]string{
		"org": "acme-corp",
	}
	tmpl := "/orgs/$org/projects/.project"
	result := renderSimple(tmpl, fields, vars)
	assert.Equal(t, "/orgs/acme-corp/projects/acme-project", result)
}

// TestDisplayJSONSummary_Object tests JSON object summary output.
func TestDisplayJSONSummary_Object(t *testing.T) {
	response := map[string]interface{}{
		"id":   "123",
		"name": "acme",
	}
	w := &bytes.Buffer{}
	displayJSONSummary(w, response)

	output := w.String()
	assert.Contains(t, output, "2 fields")
	assert.Contains(t, output, "id")
	assert.Contains(t, output, "name")
}

// TestDisplayJSONSummary_Array tests JSON array summary output.
func TestDisplayJSONSummary_Array(t *testing.T) {
	response := []interface{}{
		map[string]interface{}{"id": "1"},
	}
	w := &bytes.Buffer{}
	displayJSONSummary(w, response)

	output := w.String()
	assert.Contains(t, output, "1 elements")
}

// TestDisplayJSONSummary_OtherType tests non-object/array type handling.
func TestDisplayJSONSummary_OtherType(t *testing.T) {
	response := "hello"
	w := &bytes.Buffer{}

	// Should not panic
	displayJSONSummary(w, response)

	output := w.String()
	assert.Contains(t, output, "hello")
}

// Helper functions for file operations

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
