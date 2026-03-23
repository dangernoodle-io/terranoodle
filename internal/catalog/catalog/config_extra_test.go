package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCatalogConfig_BothAttrs(t *testing.T) {
	dir := t.TempDir()

	content := `config {
  ignore_deps    = ["infra", "networking"]
  name_must_match = "service_name"
}`
	err := os.WriteFile(filepath.Join(dir, "terra-generate.hcl"), []byte(content), 0o644)
	require.NoError(t, err)

	cfg, err := ParseCatalogConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"infra", "networking"}, cfg.IgnoreDeps)
	assert.Equal(t, "service_name", cfg.NameMustMatch)
}

func TestParseCatalogConfig_EmptyBlock(t *testing.T) {
	dir := t.TempDir()

	content := `config {
}`
	err := os.WriteFile(filepath.Join(dir, "terra-generate.hcl"), []byte(content), 0o644)
	require.NoError(t, err)

	cfg, err := ParseCatalogConfig(dir)
	require.NoError(t, err)
	assert.Empty(t, cfg.IgnoreDeps)
	assert.Empty(t, cfg.NameMustMatch)
}

func TestParseCatalogConfig_MalformedHCL(t *testing.T) {
	dir := t.TempDir()

	content := `config { this is not valid hcl !!!`
	err := os.WriteFile(filepath.Join(dir, "terra-generate.hcl"), []byte(content), 0o644)
	require.NoError(t, err)

	_, err = ParseCatalogConfig(dir)
	assert.Error(t, err)
}
