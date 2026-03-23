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
	assert.Equal(t, "vpc_id", result[0])
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
	assert.Contains(t, result, "vpc_id")
	assert.Contains(t, result, "subnet_ids")
	assert.Contains(t, result, "security_group_id")
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
	assert.Contains(t, result, "acme_vpc_id")
	assert.Contains(t, result, "acme_instance_id")
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
	assert.Equal(t, "acme_id", result[0])
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
	assert.Equal(t, "acme_output", result[0])
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
	assert.Equal(t, "acme_cluster_info", result[0])
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
	assert.Contains(t, result, "acme_region")
	assert.Contains(t, result, "acme_environment")
}
