package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCatalogConfig(t *testing.T) {
	dir := t.TempDir()

	content := `config {
  ignore_deps = ["project"]
  name_must_match = "tenant_friendly_id"
}`
	err := os.WriteFile(filepath.Join(dir, "terra-generate.hcl"), []byte(content), 0o644)
	require.NoError(t, err)

	cfg, err := ParseCatalogConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"project"}, cfg.IgnoreDeps)
	assert.Equal(t, "tenant_friendly_id", cfg.NameMustMatch)
}

func TestParseCatalogConfigMissing(t *testing.T) {
	dir := t.TempDir()

	cfg, err := ParseCatalogConfig(dir)
	require.NoError(t, err)
	assert.Empty(t, cfg.IgnoreDeps)
}
