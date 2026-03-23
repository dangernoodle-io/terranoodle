package scaffold

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFormatToTemplate_ValidFormat(t *testing.T) {
	fields := map[string]string{
		"project": "my-project",
		"service": "compute.googleapis.com",
	}
	format := "projects/{project}/services/{service}"
	result := FormatToTemplate(format, fields)

	if !strings.Contains(result, "{{ .project }}") {
		t.Errorf("expected {{ .project }} in result, got: %q", result)
	}
	if !strings.Contains(result, "{{ .service }}") {
		t.Errorf("expected {{ .service }} in result, got: %q", result)
	}
	// Should not contain the original placeholder syntax.
	if strings.Contains(result, "{project}") {
		t.Errorf("expected {project} to be replaced, got: %q", result)
	}
}

func TestFormatToTemplate_UnknownField(t *testing.T) {
	fields := map[string]string{
		"project": "my-project",
	}
	format := "projects/{project}/regions/{region}/resources/{name}"
	result := FormatToTemplate(format, fields)

	if !strings.Contains(result, "{{ .project }}") {
		t.Errorf("expected {{ .project }} in result, got: %q", result)
	}
	// Unknown fields should become TODO(fieldname).
	if !strings.Contains(result, "TODO(region)") {
		t.Errorf("expected TODO(region) in result, got: %q", result)
	}
	if !strings.Contains(result, "TODO(name)") {
		t.Errorf("expected TODO(name) in result, got: %q", result)
	}
}

func TestFormatToTemplate_Empty(t *testing.T) {
	// Should not panic on empty input.
	result := FormatToTemplate("", nil)
	if result != "" {
		t.Errorf("expected empty result for empty format, got: %q", result)
	}
}

func TestFormatToTemplate_DoubleBracePlaceholder(t *testing.T) {
	fields := map[string]string{
		"project": "proj",
	}
	// Both {{name}} and {name} styles should be handled.
	format := "{{project}}/resource"
	result := FormatToTemplate(format, fields)
	if !strings.Contains(result, "{{ .project }}") {
		t.Errorf("expected {{ .project }} in result from double-brace format, got: %q", result)
	}
}

func TestProviderFromType_KnownType(t *testing.T) {
	result := ProviderFromType("google_project_service")
	if result != "google" {
		t.Errorf("expected provider %q, got %q", "google", result)
	}
}

func TestProviderFromType_MultiPart(t *testing.T) {
	result := ProviderFromType("aws_s3_bucket")
	if result != "aws" {
		t.Errorf("expected provider %q, got %q", "aws", result)
	}
}

func TestProviderFromType_NoUnderscore(t *testing.T) {
	// Resource type with no underscore — should return the full string.
	result := ProviderFromType("resource")
	if result != "resource" {
		t.Errorf("expected %q for no-underscore type, got %q", "resource", result)
	}
}

func TestParseImportSection_StandardFormat(t *testing.T) {
	markdown := `# google_compute_instance

Some description.

## Import

` + "```" + `
terraform import google_compute_instance.default projects/my-project/zones/us-central1-a/instances/my-instance
` + "```" + `
`
	result := ParseImportSection(markdown)
	want := "projects/my-project/zones/us-central1-a/instances/my-instance"
	if result != want {
		t.Errorf("expected %q, got %q", want, result)
	}
}

func TestParseImportSection_NoImportSection(t *testing.T) {
	markdown := `# google_compute_instance

Some description with no import heading.
`
	result := ParseImportSection(markdown)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestParseImportSection_NoImportCommand(t *testing.T) {
	markdown := `# google_compute_instance

## Import

This resource cannot be imported.
`
	result := ParseImportSection(markdown)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
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
	if result != want {
		t.Errorf("expected %q, got %q", want, result)
	}
}

func TestFetchImportFormat_SuffixHit(t *testing.T) {
	importBody := "## Import\n\n" + "```" + "\nterraform import google_compute_instance.default projects/my-project/zones/us-central1-a/instances/my-instance\n" + "```"

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
	want := "projects/my-project/zones/us-central1-a/instances/my-instance"
	if result != want {
		t.Errorf("expected %q, got %q", want, result)
	}
}
