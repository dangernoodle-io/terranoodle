package scaffold

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatToTemplate_ValidFormat(t *testing.T) {
	fields := map[string]string{
		"project": "acme-project",
		"service": "compute.googleapis.com",
	}
	format := "projects/{project}/services/{service}"
	result := FormatToTemplate(format, fields)

	assert.Contains(t, result, "{{ .project }}")
	assert.Contains(t, result, "{{ .service }}")
	assert.NotContains(t, result, "{project}")
}

func TestFormatToTemplate_UnknownField(t *testing.T) {
	fields := map[string]string{
		"project": "acme-project",
	}
	format := "projects/{project}/regions/{region}/resources/{name}"
	result := FormatToTemplate(format, fields)

	assert.Contains(t, result, "{{ .project }}")
	assert.Contains(t, result, "TODO(region)")
	assert.Contains(t, result, "TODO(name)")
}

func TestFormatToTemplate_Empty(t *testing.T) {
	// Should not panic on empty input.
	result := FormatToTemplate("", nil)
	assert.Empty(t, result)
}

func TestFormatToTemplate_DoubleBracePlaceholder(t *testing.T) {
	fields := map[string]string{
		"project": "acme-project",
	}
	// Both {{name}} and {name} styles should be handled.
	format := "{{project}}/resource"
	result := FormatToTemplate(format, fields)
	assert.Contains(t, result, "{{ .project }}")
}

func TestProviderFromType_KnownType(t *testing.T) {
	result := ProviderFromType("google_project_service")
	assert.Equal(t, "google", result)
}

func TestProviderFromType_MultiPart(t *testing.T) {
	result := ProviderFromType("aws_s3_bucket")
	assert.Equal(t, "aws", result)
}

func TestProviderFromType_NoUnderscore(t *testing.T) {
	// Resource type with no underscore — should return the full string.
	result := ProviderFromType("resource")
	assert.Equal(t, "resource", result)
}

func TestParseImportSection_StandardFormat(t *testing.T) {
	markdown := `# google_compute_instance

Some description.

## Import

` + "```" + `
terraform import google_compute_instance.default projects/acme-project/zones/us-central1-a/instances/acme-instance
` + "```" + `
`
	result := ParseImportSection(markdown)
	want := "projects/acme-project/zones/us-central1-a/instances/acme-instance"
	assert.Equal(t, want, result)
}

func TestParseImportSection_NoImportSection(t *testing.T) {
	markdown := `# google_compute_instance

Some description with no import heading.
`
	result := ParseImportSection(markdown)
	assert.Empty(t, result)
}

func TestParseImportSection_NoImportCommand(t *testing.T) {
	markdown := `# google_compute_instance

## Import

This resource cannot be imported.
`
	result := ParseImportSection(markdown)
	assert.Empty(t, result)
}

func TestFetchImportFormat_FallbackURL(t *testing.T) {
	importBody := "## Import\n\n" + "```" + "\nterraform import google_project_service.default {{project}}/{{service}}\n" + "```"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/r/project_service.html.markdown"):
			http.NotFound(w, r)
		case strings.Contains(path, "/resources/project_service.md"):
			http.NotFound(w, r)
		case strings.Contains(path, "/r/google_project_service.html.markdown"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, importBody)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	original := registryBaseURL
	registryBaseURL = srv.URL
	t.Cleanup(func() { registryBaseURL = original })

	cache := map[string]string{}
	result := FetchImportFormat(context.Background(), "google_project_service", cache)
	want := "{{project}}/{{service}}"
	assert.Equal(t, want, result)
}

func TestFetchImportFormat_SuffixHit(t *testing.T) {
	importBody := "## Import\n\n" + "```" + "\nterraform import google_compute_instance.default projects/acme-project/zones/us-central1-a/instances/acme-instance\n" + "```"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/r/compute_instance.html.markdown") {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, importBody)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	original := registryBaseURL
	registryBaseURL = srv.URL
	t.Cleanup(func() { registryBaseURL = original })

	cache := map[string]string{}
	result := FetchImportFormat(context.Background(), "google_compute_instance", cache)
	want := "projects/acme-project/zones/us-central1-a/instances/acme-instance"
	assert.Equal(t, want, result)
}

func TestRenderYAML_NullField(t *testing.T) {
	var buf strings.Builder
	ti := TypeInfo{
		ResourceType: "aws_example",
		Fields: map[string]string{
			"name": "test-val",
			"opt":  "",
		},
		IDTemplate: "TODO",
	}

	err := RenderYAML(&buf, []TypeInfo{ti})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, ".opt = (null)")
	assert.Contains(t, output, ".name = \"test-val\"")
}

func TestRenderYAML_Empty(t *testing.T) {
	var buf strings.Builder

	err := RenderYAML(&buf, nil)
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "# No resource types found")
}

func TestRenderYAML_EmptySlice(t *testing.T) {
	var buf strings.Builder

	err := RenderYAML(&buf, []TypeInfo{})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "# No resource types found")
}
