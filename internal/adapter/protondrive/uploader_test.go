package protondrive

import (
	"context"
	"errors"
	"testing"

	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploader_RemotePath(t *testing.T) {
	u := NewUploader()

	tests := []struct {
		name   string
		prefix string
		subdir string
		want   string
	}{
		{
			name:   "default style",
			prefix: "/my-files/homelab-backups",
			subdir: "sonarr",
			want:   "/my-files/homelab-backups/sonarr",
		},
		{
			name:   "prefix without leading slash",
			prefix: "my-files/homelab-backups",
			subdir: "radarr",
			want:   "/my-files/homelab-backups/radarr",
		},
		{
			name:   "subdir trimmed",
			prefix: "/my-files/backups",
			subdir: "/komodo/",
			want:   "/my-files/backups/komodo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.App.RemotePrefix = tt.prefix
			assert.Equal(t, tt.want, u.RemotePath(tt.subdir))
		})
	}
}

func TestUploader_Upload_CreatesMissingFolders(t *testing.T) {
	config.App.ProtonDriveBin = "proton-drive"
	config.App.ProtonDriveDBus = false

	var calls [][]string
	u := &Uploader{run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		require.Equal(t, "proton-drive", name)
		calls = append(calls, append([]string{}, args...))

		joined := args[0] + " " + args[1]
		switch {
		case joined == "filesystem list":
			return []byte(`[]`), nil
		case joined == "filesystem info" && args[2] == "/my-files/homelab-backups":
			return nil, errors.New("not found")
		case joined == "filesystem create-folder" && args[2] == "/my-files" && args[3] == "homelab-backups":
			return []byte(`{}`), nil
		case joined == "filesystem info" && args[2] == "/my-files/homelab-backups/sonarr":
			return nil, errors.New("not found")
		case joined == "filesystem create-folder" && args[2] == "/my-files/homelab-backups" && args[3] == "sonarr":
			return []byte(`{}`), nil
		case joined == "filesystem upload":
			return []byte(`{"ok":true}`), nil
		default:
			if joined == "filesystem info" {
				return []byte(`{}`), nil
			}
			return nil, errors.New("unexpected: " + joined)
		}
	}}

	err := u.Upload(context.Background(), "/tmp/sonarr.zip", "/my-files/homelab-backups/sonarr")
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(calls), 3)
	last := calls[len(calls)-1]
	assert.Equal(t, []string{
		"filesystem", "upload",
		"--conflict-strategy", "skip",
		"--json",
		"/tmp/sonarr.zip",
		"/my-files/homelab-backups/sonarr",
	}, last)
}

func TestUploader_Upload_WrapsDBus(t *testing.T) {
	config.App.ProtonDriveBin = "/usr/local/bin/proton-drive"
	config.App.ProtonDriveDBus = true

	u := &Uploader{run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		assert.Equal(t, "dbus-run-session", name)
		assert.Equal(t, []string{"--", "/usr/local/bin/proton-drive", "filesystem", "info", "/my-files/backups"}, args)
		return []byte(`{}`), nil
	}}

	assert.True(t, u.pathExists(context.Background(), "/my-files/backups"))
}

func TestUploader_EnsureAuth_LogsURLAndWaits(t *testing.T) {
	config.App.ProtonDriveBin = "proton-drive"
	config.App.ProtonDriveDBus = false

	listCalls := 0
	u := &Uploader{
		run: func(_ context.Context, name string, args ...string) ([]byte, error) {
			require.Equal(t, "proton-drive", name)
			if args[0] == "filesystem" && args[1] == "list" {
				listCalls++
				if listCalls == 1 {
					return []byte("You need to login first"), errors.New("exit 1")
				}
				return []byte(`[]`), nil
			}
			return nil, errors.New("unexpected")
		},
		runStream: func(_ context.Context, name string, args []string, onLine func(string)) error {
			assert.Equal(t, "proton-drive", name)
			assert.Equal(t, []string{"auth", "login"}, args)
			onLine("Sign in in your browser. Keep the terminal open.")
			onLine("Open following URL manually (can be on another device) if browser did not open automatically:")
			onLine("https://account.proton.me/desktop/login?app=drive&pv=3#payload=test-token")
			return nil
		},
	}

	require.NoError(t, u.EnsureAuth(context.Background()))
	assert.Equal(t, 2, listCalls)
}

func TestLooksLikeAuthRequired(t *testing.T) {
	t.Parallel()
	assert.True(t, looksLikeAuthRequired([]byte("You need to login first"), errors.New("exit status 1")))
	assert.True(t, looksLikeAuthRequired([]byte("Failed to load session from secrets"), errors.New("exit status 1")))
	assert.False(t, looksLikeAuthRequired([]byte("permission denied"), errors.New("exit status 1")))
}

func TestExtractAuthURL(t *testing.T) {
	t.Parallel()
	url := extractAuthURL("Open this: https://account.proton.me/desktop/login?app=drive&pv=3#payload=abc")
	assert.Equal(t, "https://account.proton.me/desktop/login?app=drive&pv=3#payload=abc", url)
}
