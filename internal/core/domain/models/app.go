package models

// AppID identifies a backup target.
type AppID string

const (
	AppSonarr    AppID = "sonarr"
	AppRadarr    AppID = "radarr"
	AppProwlarr  AppID = "prowlarr"
	AppProfilarr AppID = "profilarr"
	AppLidarr    AppID = "lidarr"
	AppBazarr    AppID = "bazarr"
	AppNavidrome AppID = "navidrome"
	AppSeerr     AppID = "seerr"
	AppSoulsync  AppID = "soulsync"
	AppIgnis     AppID = "ignis"
	AppKomodo    AppID = "komodo"
	AppPostgres  AppID = "postgres"
)

// Kind describes how a target is backed up.
type Kind string

const (
	KindConfigSync    Kind = "config_sync"
	KindArrAPI        Kind = "arr_api"
	KindMongoDump     Kind = "mongo_dump"
	KindPostgresDump  Kind = "postgres_dump"
)

// APIScheme identifies which HTTP backup API to use.
type APIScheme string

const (
	APISchemeServarrV1 APIScheme = "servarr_v1" // Prowlarr, Lidarr
	APISchemeServarrV3 APIScheme = "servarr_v3" // Sonarr, Radarr
	APISchemeBazarr    APIScheme = "bazarr"
)

// BackupTarget is a resolved enabled backup job.
type BackupTarget struct {
	ID           AppID
	Kind         Kind
	LocalPath    string
	RemoteSubdir string
	APIBaseURL   string
	APIKey       string
	BackupMount  string
	APIScheme    APIScheme
}

// BackupResult summarizes one target run.
type BackupResult struct {
	Target AppID
	Err    error
}

func (r BackupResult) Succeeded() bool {
	return r.Err == nil
}
