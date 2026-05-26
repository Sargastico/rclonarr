package config

const AppTraceName = "rclonarr"

// AppInfo is populated from APP_* environment variables (see envconfig tags).
// Note: envconfig's default key for RemoteName is APP_REMOTENAME, not APP_REMOTE_NAME.
type AppInfo struct {
	ReleaseVersion         string `default:"rclonarr" envconfig:"RELEASE_VERSION"`
	Environment            string `default:"local" envconfig:"ENVIRONMENT"`
	Development            bool   `default:"true" envconfig:"DEVELOPMENT"`
	Debug                  bool   `default:"true" envconfig:"DEBUG"`
	OtelExporterEndpoint   string `default:"localhost:4317" envconfig:"OTEL_EXPORTER_ENDPOINT"`
	EnableOtel             bool   `default:"false" envconfig:"ENABLE_OTEL"`
	EnableProfiling        bool   `default:"false" envconfig:"ENABLE_PROFILING"`
	PyroscopeServerAddress string `default:"http://localhost:4040" envconfig:"PYROSCOPE_SERVER_ADDRESS"`

	RcloneConfigPath string `default:"" envconfig:"RCLONE_CONFIG_PATH"`
	RemoteName       string `default:"" envconfig:"REMOTE_NAME"`
	RemotePrefix     string `default:"homelab-backups" envconfig:"REMOTE_PREFIX"`

	EnabledTargets string `default:"" envconfig:"ENABLED_TARGETS"`

	// Schedule is a standard 5-field cron expression (e.g. "0 3 * * *"). Empty = run once and exit.
	// When set, daemon mode runs one bootstrap backup, then waits for cron.
	Schedule string `default:"" envconfig:"SCHEDULE"`

	CommandPollIntervalMS int `default:"1000" envconfig:"COMMAND_POLL_INTERVAL_MS"`
	CommandTimeoutSec     int `default:"300" envconfig:"COMMAND_TIMEOUT_SEC"`

	SonarrURL         string `default:"" envconfig:"SONARR_URL"`
	SonarrAPIKey      string `default:"" envconfig:"SONARR_API_KEY"`
	SonarrBackupMount string `default:"" envconfig:"SONARR_BACKUP_MOUNT"`
	SonarrConfigPath  string `default:"" envconfig:"SONARR_CONFIG_PATH"`

	RadarrURL         string `default:"" envconfig:"RADARR_URL"`
	RadarrAPIKey      string `default:"" envconfig:"RADARR_API_KEY"`
	RadarrBackupMount string `default:"" envconfig:"RADARR_BACKUP_MOUNT"`
	RadarrConfigPath  string `default:"" envconfig:"RADARR_CONFIG_PATH"`

	ProwlarrURL         string `default:"" envconfig:"PROWLARR_URL"`
	ProwlarrAPIKey      string `default:"" envconfig:"PROWLARR_API_KEY"`
	ProwlarrBackupMount string `default:"" envconfig:"PROWLARR_BACKUP_MOUNT"`
	ProwlarrConfigPath  string `default:"" envconfig:"PROWLARR_CONFIG_PATH"`

	ProfilarrConfigPath string `default:"" envconfig:"PROFILARR_CONFIG_PATH"`

	LidarrURL         string `default:"" envconfig:"LIDARR_URL"`
	LidarrAPIKey      string `default:"" envconfig:"LIDARR_API_KEY"`
	LidarrBackupMount string `default:"" envconfig:"LIDARR_BACKUP_MOUNT"`
	LidarrConfigPath  string `default:"" envconfig:"LIDARR_CONFIG_PATH"`

	BazarrURL         string `default:"" envconfig:"BAZARR_URL"`
	BazarrAPIKey      string `default:"" envconfig:"BAZARR_API_KEY"`
	BazarrBackupMount string `default:"" envconfig:"BAZARR_BACKUP_MOUNT"`
	BazarrConfigPath  string `default:"" envconfig:"BAZARR_CONFIG_PATH"`

	NavidromeConfigPath string `default:"" envconfig:"NAVIDROME_CONFIG_PATH"`
	SeerrConfigPath     string `default:"" envconfig:"SEERR_CONFIG_PATH"`
	SoulsyncConfigPath  string `default:"" envconfig:"SOULSYNC_CONFIG_PATH"`

	KomodoMongoURI      string `default:"" envconfig:"KOMODO_MONGO_URI"`
	KomodoMongoAddress  string `default:"" envconfig:"KOMODO_MONGO_ADDRESS"`
	KomodoMongoUsername string `default:"" envconfig:"KOMODO_MONGO_USERNAME"`
	KomodoMongoPassword string `default:"" envconfig:"KOMODO_MONGO_PASSWORD"`
	KomodoMongoDBName   string `default:"komodo" envconfig:"KOMODO_MONGO_DB_NAME"`
	KomodoDumpDir       string `default:"" envconfig:"KOMODO_DUMP_DIR"`
	KomodoDumpExtraArgs string `default:"" envconfig:"KOMODO_DUMP_EXTRA_ARGS"`
	MongodumpPath       string `default:"mongodump" envconfig:"MONGODUMP_PATH"`
}

var App AppInfo
