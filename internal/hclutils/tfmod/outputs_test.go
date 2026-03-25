package tfmod

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOutputs_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	result, err := ParseOutputs(dir)

	require.Nil(t, err)
	assert.Nil(t, result)
}

func TestParseOutputs_SingleOutput(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "outputs.tf")
	content := `output "vpc_id" { value = "acme-vpc-123" }`
	require.Nil(t, os.WriteFile(outputFile, []byte(content), 0o644))

	result, err := ParseOutputs(dir)

	require.Nil(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "vpc_id", result[0].Name)
}

func TestParseOutputs_MultipleOutputsInOneFile(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "outputs.tf")
	content := `output "vpc_id" {
  value = "acme-vpc-123"
}

output "subnet_ids" {
  value = ["acme-subnet-1", "acme-subnet-2"]
}

output "security_group_id" {
  value = "acme-sg-456"
}`
	require.Nil(t, os.WriteFile(outputFile, []byte(content), 0o644))

	result, err := ParseOutputs(dir)

	require.Nil(t, err)
	require.Len(t, result, 3)
	names := make(map[string]bool)
	for _, o := range result {
		names[o.Name] = true
	}
	assert.True(t, names["vpc_id"])
	assert.True(t, names["subnet_ids"])
	assert.True(t, names["security_group_id"])
}

func TestParseOutputs_MultipleFiles(t *testing.T) {
	dir := t.TempDir()

	// First file
	outputFile1 := filepath.Join(dir, "outputs.tf")
	content1 := `output "acme_vpc_id" {
  value = "acme-vpc-123"
}`
	require.Nil(t, os.WriteFile(outputFile1, []byte(content1), 0o644))

	// Second file
	outputFile2 := filepath.Join(dir, "local_outputs.tf")
	content2 := `output "acme_instance_id" {
  value = "acme-instance-456"
}`
	require.Nil(t, os.WriteFile(outputFile2, []byte(content2), 0o644))

	result, err := ParseOutputs(dir)

	require.Nil(t, err)
	require.Len(t, result, 2)
	names := make(map[string]bool)
	for _, o := range result {
		names[o.Name] = true
	}
	assert.True(t, names["acme_vpc_id"])
	assert.True(t, names["acme_instance_id"])
}

func TestParseOutputs_NonTfFilesIgnored(t *testing.T) {
	dir := t.TempDir()

	// Write a .tf file
	tfFile := filepath.Join(dir, "outputs.tf")
	content := `output "acme_id" { value = "123" }`
	require.Nil(t, os.WriteFile(tfFile, []byte(content), 0o644))

	// Write a .hcl file (should be ignored)
	hclFile := filepath.Join(dir, "other.hcl")
	hclContent := `output "ignored_output" { value = "456" }`
	require.Nil(t, os.WriteFile(hclFile, []byte(hclContent), 0o644))

	result, err := ParseOutputs(dir)

	require.Nil(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "acme_id", result[0].Name)
}

func TestParseOutputs_DirectoriesIgnored(t *testing.T) {
	dir := t.TempDir()

	// Write a .tf file
	tfFile := filepath.Join(dir, "outputs.tf")
	content := `output "acme_output" { value = "123" }`
	require.Nil(t, os.WriteFile(tfFile, []byte(content), 0o644))

	// Create a subdirectory (should be ignored)
	subDir := filepath.Join(dir, "subdir.tf")
	require.Nil(t, os.Mkdir(subDir, 0o755))

	result, err := ParseOutputs(dir)

	require.Nil(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "acme_output", result[0].Name)
}

func TestParseOutputs_NonexistentDir(t *testing.T) {
	_, err := ParseOutputs("/nonexistent/path/to/acme-module")

	require.NotNil(t, err)
}

func TestParseOutputs_ComplexOutput(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "outputs.tf")
	content := `output "acme_cluster_info" {
  value = {
    endpoint     = "acme-cluster.us-west-2.rds.amazonaws.com"
    port         = 5432
    database     = "staging"
    tags         = { "Environment" = "staging", "Team" = "acme" }
  }
  description = "RDS cluster information"
}`
	require.Nil(t, os.WriteFile(outputFile, []byte(content), 0o644))

	result, err := ParseOutputs(dir)

	require.Nil(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "acme_cluster_info", result[0].Name)
}

func TestParseOutputs_MixedContent(t *testing.T) {
	dir := t.TempDir()

	// File with variables and outputs (only outputs should be extracted)
	mixedFile := filepath.Join(dir, "mixed.tf")
	content := `variable "region" {
  type    = string
  default = "us-west-2"
}

output "acme_region" {
  value = var.region
}

output "acme_environment" {
  value = "staging"
}`
	require.Nil(t, os.WriteFile(mixedFile, []byte(content), 0o644))

	result, err := ParseOutputs(dir)

	require.Nil(t, err)
	require.Len(t, result, 2)
	names := make(map[string]bool)
	for _, o := range result {
		names[o.Name] = true
	}
	assert.True(t, names["acme_region"])
	assert.True(t, names["acme_environment"])
}

func TestParseOutputs_HasDescription(t *testing.T) {
	dir := t.TempDir()
	writeFile := func(t *testing.T, dir, filename, content string) {
		require.Nil(t, os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644))
	}
	writeFile(t, dir, "outputs.tf", `
output "with_desc" {
  value       = "test"
  description = "has a description"
}

output "without_desc" {
  value = "test"
}
`)
	result, err := ParseOutputs(dir)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "with_desc", result[0].Name)
	assert.True(t, result[0].HasDescription)
	assert.Equal(t, "without_desc", result[1].Name)
	assert.False(t, result[1].HasDescription)
}
