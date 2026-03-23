package hclparse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeHCL(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "template.hcl")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
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
	require.NoError(t, err)
	assert.Empty(t, warnings)
	require.Len(t, def.Stacks, 1)
	stack := def.Stacks[0]
	assert.Equal(t, "my-service", stack.Name)
	assert.Contains(t, stack.Values, "service")
	assert.Contains(t, stack.Values, "env")
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
	require.NoError(t, err)
	require.Len(t, def.Stacks, 2)
	assert.Equal(t, "service-a", def.Stacks[0].Name)
	assert.Equal(t, "service-b", def.Stacks[1].Name)
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
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "duplicate")
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
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "empty")
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
	require.NoError(t, err)
	require.Len(t, def.IgnoreDeps, 2)
	assert.Equal(t, "foo", def.IgnoreDeps[0])
	assert.Equal(t, "bar", def.IgnoreDeps[1])
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
	require.NoError(t, err)
	assert.Equal(t, "service", def.NameMustMatch)
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
	require.NoError(t, err)
	require.NotNil(t, def.Locals)
	assert.Contains(t, def.Locals, "env")
	// The value should have been resolved via local.env reference.
	require.Len(t, def.Stacks, 1)
	assert.Contains(t, def.Stacks[0].Values, "environment")
}

func TestParseTemplateFile_MissingFile(t *testing.T) {
	_, _, err := ParseTemplateFile("/nonexistent/path/template.hcl")
	require.Error(t, err)
}

func TestParseTemplateFile_ConfigIgnoreDeps_NonStringElement(t *testing.T) {
	dir := t.TempDir()
	path := writeHCL(t, dir, `
config {
  ignore_deps = ["valid", 42]
}

template "svc" {
  values = {
    name = "svc"
  }
}
`)
	_, _, err := ParseTemplateFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ignore_deps elements must be strings")
}
