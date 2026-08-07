package config

import (
	"testing"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/require"
)

func TestEnvconfigUsesUnderscoreKeys(t *testing.T) {
	t.Setenv("APP_REMOTE_PREFIX", "/my-files/homelab/backups")
	t.Setenv("APP_PROTON_DRIVE_BIN", "/usr/local/bin/proton-drive")
	t.Setenv("APP_PROTON_DRIVE_DBUS", "true")
	t.Setenv("APP_ENABLED_TARGETS", "sonarr")

	App = AppInfo{}
	require.NoError(t, envconfig.Process("app", &App))
	require.Equal(t, "/my-files/homelab/backups", App.RemotePrefix)
	require.Equal(t, "/usr/local/bin/proton-drive", App.ProtonDriveBin)
	require.True(t, App.ProtonDriveDBus)
	require.Equal(t, "sonarr", App.EnabledTargets)
}
