package servarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/Sargastico/rclonarr/internal/core/domain/models"
	"github.com/stretchr/testify/require"
)

func TestClient_Trigger(t *testing.T) {
	t.Parallel()

	backupDir := t.TempDir()
	backupSubdir := filepath.Join(backupDir, "Backups", "scheduled")
	require.NoError(t, os.MkdirAll(backupSubdir, 0o755))
	backupFile := filepath.Join(backupSubdir, "sonarr_backup.zip")
	require.NoError(t, os.WriteFile(backupFile, []byte("zip"), 0o600))

	var commandID int64 = 1
	listCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/system/backup":
			listCalls++
			backups := []backupResource{}
			if listCalls > 1 {
				backups = []backupResource{{
					ID:   2,
					Name: "sonarr_backup.zip",
					Path: "/config/Backups/scheduled/sonarr_backup.zip",
					Time: "2026-05-26T12:00:00Z",
				}}
			}
			require.NoError(t, json.NewEncoder(w).Encode(backups))
		case "/api/v3/command":
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, json.NewEncoder(w).Encode(commandResponse{ID: commandID, Name: commandBackup, Status: "queued"}))
		case "/api/v3/command/1":
			require.NoError(t, json.NewEncoder(w).Encode(commandResponse{ID: commandID, Status: statusCompleted}))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	config.App = config.AppInfo{
		CommandPollIntervalMS: 10,
		CommandTimeoutSec:     5,
	}

	client := NewClient(srv.Client())
	target := models.BackupTarget{
		ID:          models.AppSonarr,
		APIScheme:   models.APISchemeServarrV3,
		APIBaseURL:  srv.URL,
		APIKey:      "test-key",
		BackupMount: backupDir,
	}

	path, err := client.Trigger(context.Background(), target)
	require.NoError(t, err)
	require.Equal(t, backupFile, path)
}

func TestResolveLocalPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sub := filepath.Join(dir, "Backups", "scheduled")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	file := filepath.Join(sub, "backup.zip")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	target := models.BackupTarget{BackupMount: dir}
	got, err := resolveLocalPath(target, "/config/Backups/scheduled/backup.zip")
	require.NoError(t, err)
	require.Equal(t, file, got)

	manualDir := filepath.Join(dir, "Backups", "manual")
	require.NoError(t, os.MkdirAll(manualDir, 0o755))
	manualFile := filepath.Join(manualDir, "sonarr_backup.zip")
	require.NoError(t, os.WriteFile(manualFile, []byte("x"), 0o600))

	got, err = resolveLocalPath(target, "/backup/manual/sonarr_backup.zip")
	require.NoError(t, err)
	require.Equal(t, manualFile, got)
}
