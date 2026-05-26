package config

import (
	"testing"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/require"
)

func TestEnvconfigUsesUnderscoreKeys(t *testing.T) {
	t.Setenv("APP_REMOTE_NAME", "protondrive-sargx")
	t.Setenv("APP_REMOTE_PREFIX", "homelab/backups")
	t.Setenv("APP_RCLONE_CONFIG_PATH", "/config/rclone/rclone.conf")
	t.Setenv("APP_ENABLED_TARGETS", "sonarr")

	App = AppInfo{}
	require.NoError(t, envconfig.Process("app", &App))
	require.Equal(t, "protondrive-sargx", App.RemoteName)
	require.Equal(t, "homelab/backups", App.RemotePrefix)
	require.Equal(t, "/config/rclone/rclone.conf", App.RcloneConfigPath)
	require.Equal(t, "sonarr", App.EnabledTargets)
}
