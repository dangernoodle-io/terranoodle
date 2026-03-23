package hclparse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeHCL(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "template.hcl")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture template.hcl: %v", err)
	}
	return path
}

func TestParseTemplateFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := writeHCL(t, dir, `
template "my-service" {
  values = {
    service = "my-service"
    env     = "prod"
  }
}
`)
	def, warnings, err := ParseTemplateFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(def.Stacks) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(def.Stacks))
	}
	stack := def.Stacks[0]
	if stack.Name != "my-service" {
		t.Errorf("expected name %q, got %q", "my-service", stack.Name)
	}
	if _, ok := stack.Values["service"]; !ok {
		t.Error("expected values key 'service' to be present")
	}
	if _, ok := stack.Values["env"]; !ok {
		t.Error("expected values key 'env' to be present")
	}
}

func TestParseTemplateFile_MultipleTemplates(t *testing.T) {
	dir := t.TempDir()
	path := writeHCL(t, dir, `
template "service-a" {
  values = {
    name = "a"
  }
}

template "service-b" {
  values = {
    name = "b"
  }
}
`)
	def, _, err := ParseTemplateFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(def.Stacks) != 2 {
		t.Fatalf("expected 2 stacks, got %d", len(def.Stacks))
	}
	if def.Stacks[0].Name != "service-a" {
		t.Errorf("expected first stack name %q, got %q", "service-a", def.Stacks[0].Name)
	}
	if def.Stacks[1].Name != "service-b" {
		t.Errorf("expected second stack name %q, got %q", "service-b", def.Stacks[1].Name)
	}
}

func TestParseTemplateFile_DuplicateName(t *testing.T) {
	dir := t.TempDir()
	path := writeHCL(t, dir, `
template "my-service" {
  values = {
    name = "first"
  }
}

template "my-service" {
  values = {
    name = "second"
  }
}
`)
	_, _, err := ParseTemplateFile(path)
	if err == nil {
		t.Fatal("expected error for duplicate template name, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		t.Errorf("expected error to contain 'duplicate', got: %v", err)
	}
}

func TestParseTemplateFile_EmptyName(t *testing.T) {
	dir := t.TempDir()
	// HCL requires at least one label for the template block; an empty string label
	// triggers the empty-name validation.
	path := writeHCL(t, dir, `
template "" {
  values = {
    name = "something"
  }
}
`)
	_, _, err := ParseTemplateFile(path)
	if err == nil {
		t.Fatal("expected error for empty template name, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "empty") {
		t.Errorf("expected error to contain 'empty', got: %v", err)
	}
}

func TestParseTemplateFile_ConfigIgnoreDeps(t *testing.T) {
	dir := t.TempDir()
	path := writeHCL(t, dir, `
config {
  ignore_deps = ["foo", "bar"]
}

template "svc" {
  values = {
    name = "svc"
  }
}
`)
	def, _, err := ParseTemplateFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(def.IgnoreDeps) != 2 {
		t.Fatalf("expected 2 ignore_deps, got %d: %v", len(def.IgnoreDeps), def.IgnoreDeps)
	}
	if def.IgnoreDeps[0] != "foo" || def.IgnoreDeps[1] != "bar" {
		t.Errorf("unexpected ignore_deps values: %v", def.IgnoreDeps)
	}
}

func TestParseTemplateFile_ConfigNameMustMatch(t *testing.T) {
	dir := t.TempDir()
	path := writeHCL(t, dir, `
config {
  name_must_match = "service"
}

template "svc" {
  values = {
    service = "svc"
  }
}
`)
	def, _, err := ParseTemplateFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.NameMustMatch != "service" {
		t.Errorf("expected NameMustMatch %q, got %q", "service", def.NameMustMatch)
	}
}

func TestParseTemplateFile_Locals(t *testing.T) {
	dir := t.TempDir()
	path := writeHCL(t, dir, `
locals {
  env = "prod"
}

template "svc" {
  values = {
    environment = local.env
  }
}
`)
	def, _, err := ParseTemplateFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.Locals == nil {
		t.Fatal("expected non-nil Locals map")
	}
	if _, ok := def.Locals["env"]; !ok {
		t.Error("expected 'env' in Locals")
	}
	// The value should have been resolved via local.env reference.
	if len(def.Stacks) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(def.Stacks))
	}
	if _, ok := def.Stacks[0].Values["environment"]; !ok {
		t.Error("expected 'environment' key in evaluated values")
	}
}

func TestParseTemplateFile_MissingFile(t *testing.T) {
	_, _, err := ParseTemplateFile("/nonexistent/path/template.hcl")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
