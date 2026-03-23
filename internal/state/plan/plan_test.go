package plan

import (
	"strings"
	"testing"
)

const validPlanJSON = `{
  "format_version": "1.0",
  "resource_changes": [
    {
      "address": "google_project_service.apis",
      "type": "google_project_service",
      "change": {"actions": ["create"]}
    },
    {
      "address": "google_project_service.existing",
      "type": "google_project_service",
      "change": {"actions": ["no-op"]}
    }
  ]
}`

const noCreatesPlanJSON = `{
  "format_version": "1.0",
  "resource_changes": [
    {
      "address": "google_project_service.existing",
      "type": "google_project_service",
      "change": {"actions": ["no-op"]}
    }
  ]
}`

func TestParse_Valid(t *testing.T) {
	p, err := Parse(strings.NewReader(validPlanJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.ResourceChanges) != 2 {
		t.Errorf("expected 2 resource changes, got %d", len(p.ResourceChanges))
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := Parse(strings.NewReader(`not valid json {{{`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestFilterCreates_OnlyCreates(t *testing.T) {
	p, err := Parse(strings.NewReader(validPlanJSON))
	if err != nil {
		t.Fatalf("unexpected error parsing plan: %v", err)
	}

	creates := FilterCreates(p)
	if len(creates) != 1 {
		t.Fatalf("expected 1 create, got %d", len(creates))
	}
	if creates[0].Address != "google_project_service.apis" {
		t.Errorf("expected address %q, got %q", "google_project_service.apis", creates[0].Address)
	}
}

func TestFilterCreates_Empty(t *testing.T) {
	p, err := Parse(strings.NewReader(noCreatesPlanJSON))
	if err != nil {
		t.Fatalf("unexpected error parsing plan: %v", err)
	}

	creates := FilterCreates(p)
	if len(creates) != 0 {
		t.Errorf("expected 0 creates, got %d", len(creates))
	}
}
