package credentials

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServarrAPIKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.xml"), []byte(`<?xml version="1.0"?>
<Config>
  <ApiKey>abc-123</ApiKey>
</Config>`), 0o600))

	got, err := ServarrAPIKey(dir)
	require.NoError(t, err)
	assert.Equal(t, "abc-123", got)
}

func TestBazarrAPIKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	backup := filepath.Join(dir, "backup")
	require.NoError(t, os.Mkdir(backup, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(`
auth:
  apikey: 'bazarr-secret'
`), 0o600))

	got, err := BazarrAPIKey(backup)
	require.NoError(t, err)
	assert.Equal(t, "bazarr-secret", got)
}

func TestBazarrAPIKey_configSubdir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	backup := filepath.Join(dir, "backup")
	configDir := filepath.Join(dir, "config")
	require.NoError(t, os.MkdirAll(backup, 0o755))
	require.NoError(t, os.Mkdir(configDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(`
auth:
  apikey: b3ac1157c99cc7d13ee0bca4b81fcc41
sonarr:
  apikey: a9ffa7f62e3d48a1a2bfbbfafb9b79f4
`), 0o600))

	got, err := BazarrAPIKey(backup)
	require.NoError(t, err)
	assert.Equal(t, "b3ac1157c99cc7d13ee0bca4b81fcc41", got)
}
