package archive

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionedZipName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		targetID string
		at       time.Time
		want     string
	}{
		{
			name:     "utc formatting",
			targetID: "sonarr",
			at:       time.Date(2026, 5, 27, 3, 10, 53, 0, time.FixedZone("BRT", -3*3600)),
			want:     "sonarr_backup_20260527_061053.zip",
		},
		{
			name:     "komodo",
			targetID: "komodo",
			at:       time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC),
			want:     "komodo_backup_20260102_120000.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, VersionedZipName(tt.targetID, tt.at))
		})
	}
}

func TestZipDirectory(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "config.xml"), []byte("cfg"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "nested", "db.sqlite"), []byte("db"), 0o644))

	dest := filepath.Join(t.TempDir(), "out.zip")
	require.NoError(t, ZipDirectory(src, dest))

	r, err := zip.OpenReader(dest)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	names := make([]string, 0, len(r.File))
	contents := map[string]string{}
	for _, f := range r.File {
		names = append(names, f.Name)
		if f.FileInfo().IsDir() {
			continue
		}
		rc, openErr := f.Open()
		require.NoError(t, openErr)
		data, readErr := io.ReadAll(rc)
		_ = rc.Close()
		require.NoError(t, readErr)
		contents[f.Name] = string(data)
	}

	assert.Contains(t, names, "config.xml")
	assert.Contains(t, names, "nested/db.sqlite")
	assert.Equal(t, "cfg", contents["config.xml"])
	assert.Equal(t, "db", contents["nested/db.sqlite"])
}

func TestZipDirectoryToTemp(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o644))

	at := time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC)
	zipPath, cleanup, err := ZipDirectoryToTemp(src, "profilarr", at)
	require.NoError(t, err)
	defer cleanup()

	assert.Equal(t, "profilarr_backup_20260527_000000.zip", filepath.Base(zipPath))
	_, statErr := os.Stat(zipPath)
	require.NoError(t, statErr)
}

func TestIsZipFile(t *testing.T) {
	t.Parallel()

	assert.True(t, IsZipFile("/backups/foo.ZIP"))
	assert.False(t, IsZipFile("/backups/foo.tar"))
}
