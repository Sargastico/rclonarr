package config

const AppTraceName = "rclonarr"

type AppInfo struct {
	ReleaseVersion         string `default:"rclonarr"`
	Environment            string `default:"local"`
	Development            bool   `default:"true"`
	Debug                  bool   `default:"true"`
	OtelExporterEndpoint   string `default:"localhost:4317"`
	EnableOtel             bool   `default:"false"`
	EnableProfiling        bool   `default:"false"`
	PyroscopeServerAddress string `default:"http://localhost:4040"`

	RcloneConfigPath string `default:""`
	RemoteName       string `default:""`
	RemotePrefix     string `default:"homelab-backups"`

	EnabledTargets string `default:""`

	// Schedule is a standard 5-field cron expression (e.g. "0 3 * * *"). Empty = run once and exit.
	// When set, daemon mode runs one bootstrap backup, then waits for cron.
	Schedule string `default:""`

	CommandPollIntervalMS int `default:"1000"`
	CommandTimeoutSec       int `default:"300"`

	SonarrURL         string `default:""`
	SonarrAPIKey      string `default:""`
	SonarrBackupMount string `default:""`
	SonarrConfigPath  string `default:""`

	RadarrURL         string `default:""`
	RadarrAPIKey      string `default:""`
	RadarrBackupMount string `default:""`
	RadarrConfigPath  string `default:""`

	ProwlarrURL         string `default:""`
	ProwlarrAPIKey      string `default:""`
	ProwlarrBackupMount string `default:""`
	ProwlarrConfigPath  string `default:""`

	ProfilarrConfigPath string `default:""`

	LidarrURL         string `default:""`
	LidarrAPIKey      string `default:""`
	LidarrBackupMount string `default:""`
	LidarrConfigPath  string `default:""`

	BazarrURL         string `default:""`
	BazarrAPIKey      string `default:""`
	BazarrBackupMount string `default:""`
	BazarrConfigPath  string `default:""`

	NavidromeConfigPath string `default:""`
	SeerrConfigPath     string `default:""`
	SoulsyncConfigPath  string `default:""`

	KomodoMongoURI      string `default:""`
	KomodoMongoAddress  string `default:""`
	KomodoMongoUsername string `default:""`
	KomodoMongoPassword string `default:""`
	KomodoMongoDBName   string `default:"komodo"`
	KomodoDumpDir       string `default:""`
	KomodoDumpExtraArgs string `default:""`
	MongodumpPath       string `default:"mongodump"`
}

var App AppInfo
