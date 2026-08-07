package protondrive

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseListJSON_WrappedName(t *testing.T) {
	t.Parallel()

	raw := []byte(`[
	  {
	    "name": {"ok": true, "value": "sonarr_backup_20260801_030000.zip"},
	    "type": "file",
	    "modificationTime": "2026-08-01T03:00:00.000Z"
	  },
	  {
	    "name": {"ok": true, "value": "subdir"},
	    "type": "folder"
	  }
	]`)

	files, err := parseListJSON(raw, "/my-files/backups/sonarr")
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "sonarr_backup_20260801_030000.zip", files[0].Name)
	assert.Equal(t, "/my-files/backups/sonarr/sonarr_backup_20260801_030000.zip", files[0].Path)
	assert.Equal(t, time.Date(2026, 8, 1, 3, 0, 0, 0, time.UTC), files[0].ModTime)
}

func TestParseListJSON_ValueWrapper(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"ok":true,"value":[{"name":"postgres_backup_20260801_030000.zip","type":"file"}]}`)
	files, err := parseListJSON(raw, "/r/postgres")
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "postgres_backup_20260801_030000.zip", files[0].Name)
}
