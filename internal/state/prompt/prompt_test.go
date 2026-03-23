package prompt

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"dangernoodle.io/terratools/internal/state/config"
	"dangernoodle.io/terratools/internal/state/resolver"
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

// TestAPIAssisted_SkipEmptyChoice tests skipping with empty line.
func TestAPIAssisted_SkipEmptyChoice(t *testing.T) {
	input := "\n"
	r := strings.NewReader(input)
	w := &bytes.Buffer{}
	client := resolver.NewAPIClient("http://unused", "")

	id, save, def, err := APIAssisted(r, w, "aws_s3_bucket.main", "aws_s3_bucket", nil, client, "http://unused", nil, nil)

	require.NoError(t, err)
	assert.Empty(t, id)
	assert.False(t, save)
	assert.Nil(t, def)
}

// TestAPIAssisted_ChoiceManual tests choice [1] delegates to manual ID.
func TestAPIAssisted_ChoiceManual(t *testing.T) {
	input := "1\nacme-bucket-123\ny\n"
	r := strings.NewReader(input)
	w := &bytes.Buffer{}
	client := resolver.NewAPIClient("http://unused", "")

	id, save, def, err := APIAssisted(r, w, "aws_s3_bucket.main", "aws_s3_bucket", nil, client, "http://unused", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "acme-bucket-123", id)
	assert.True(t, save)
	assert.Nil(t, def)
}

// TestAPIAssisted_UnknownChoice tests unknown choice output.
func TestAPIAssisted_UnknownChoice(t *testing.T) {
	input := "9\n"
	r := strings.NewReader(input)
	w := &bytes.Buffer{}
	client := resolver.NewAPIClient("http://unused", "")

	id, save, def, err := APIAssisted(r, w, "aws_s3_bucket.main", "aws_s3_bucket", nil, client, "http://unused", nil, nil)

	require.NoError(t, err)
	assert.Empty(t, id)
	assert.False(t, save)
	assert.Nil(t, def)
	assert.Contains(t, w.String(), "Unknown choice")
}

// TestAPIAssisted_ChoiceResolverObjectResponse tests building resolver with object response.
func TestAPIAssisted_ChoiceResolverObjectResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "123",
			"name": "acme-badge",
			"slug": "acme-slug",
		})
	}))
	defer srv.Close()
	client := resolver.NewAPIClient(srv.URL, "test-token")

	input := "2\nbadge_id\n\n/api/v1/badges\nslug\n$badge_id\ny\n"
	r := strings.NewReader(input)
	w := &bytes.Buffer{}

	id, save, def, err := APIAssisted(r, w, "google_project_badge.main", "google_project_badge", nil, client, srv.URL, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "$badge_id", id)
	assert.True(t, save)
	assert.NotNil(t, def)
	assert.Equal(t, "badge_id", def.Name)
	assert.Equal(t, "/api/v1/badges", def.Get)
	assert.Equal(t, "slug", def.Pick)
}

// TestAPIAssisted_ChoiceResolverArrayResponse tests building resolver with array response.
func TestAPIAssisted_ChoiceResolverArrayResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]interface{}{
			map[string]interface{}{"id": "1", "name": "alpha"},
			map[string]interface{}{"id": "2", "name": "beta"},
		})
	}))
	defer srv.Close()
	client := resolver.NewAPIClient(srv.URL, "test-token")

	input := "2\nlookup_id\n\n/api/v1/items\nname\n.name\nid\n$lookup_id\ny\n"
	r := strings.NewReader(input)
	w := &bytes.Buffer{}

	id, save, def, err := APIAssisted(r, w, "acme_widget.main", "acme_widget", nil, client, srv.URL, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "$lookup_id", id)
	assert.True(t, save)
	assert.NotNil(t, def)
	assert.Equal(t, "lookup_id", def.Name)
	assert.Equal(t, "/api/v1/items", def.Get)

	// Verify Pick is a map with where and field keys
	pickMap, ok := def.Pick.(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, pickMap, "where")
	assert.Contains(t, pickMap, "field")
}

// TestAPIAssisted_EmptyResolverName tests resolver with empty name is skipped.
func TestAPIAssisted_EmptyResolverName(t *testing.T) {
	input := "2\n\n"
	r := strings.NewReader(input)
	w := &bytes.Buffer{}
	client := resolver.NewAPIClient("http://unused", "")

	id, save, def, err := APIAssisted(r, w, "aws_s3_bucket.main", "aws_s3_bucket", nil, client, "http://unused", nil, nil)

	require.NoError(t, err)
	assert.Empty(t, id)
	assert.False(t, save)
	assert.Nil(t, def)
	assert.Contains(t, w.String(), "No resolver name")
}

// TestFormatResolverYAML_StringPick tests YAML formatting with string pick.
func TestFormatResolverYAML_StringPick(t *testing.T) {
	result := &ResolverResult{
		Name: "badge_id",
		Get:  "/api/badges",
		Pick: "slug",
		Use:  nil,
		TypeMapping: config.TypeMapping{
			Use: []string{"badge_id"},
			ID:  "$badge_id",
		},
	}
	w := &bytes.Buffer{}

	formatResolverYAML(w, result, "google_project_badge")
	output := w.String()

	assert.Contains(t, output, "badge_id:")
	assert.Contains(t, output, `get: "/api/badges"`)
	assert.Contains(t, output, `pick: "slug"`)
}

// TestFormatResolverYAML_MapPickWithDeps tests YAML formatting with map pick and dependencies.
func TestFormatResolverYAML_MapPickWithDeps(t *testing.T) {
	result := &ResolverResult{
		Name: "lookup",
		Get:  "/api/items",
		Pick: map[string]interface{}{
			"where": map[string]string{"name": ".name"},
			"field": "id",
		},
		Use: []string{"other_resolver"},
		TypeMapping: config.TypeMapping{
			Use: []string{"other_resolver", "lookup"},
			ID:  "$lookup",
		},
	}
	w := &bytes.Buffer{}

	formatResolverYAML(w, result, "acme_widget")
	output := w.String()

	assert.Contains(t, output, "lookup:")
	assert.Contains(t, output, "use: [other_resolver]")
	assert.Contains(t, output, `get: "/api/items"`)
	assert.Contains(t, output, "pick:")
}

// Helper functions for file operations

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
