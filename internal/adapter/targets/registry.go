package targets

import (
	"os"
	"strings"

	"github.com/Sargastico/rclonarr/internal/adapter/credentials"
	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/Sargastico/rclonarr/internal/core/domain/models"
	"github.com/Sargastico/rclonarr/internal/core/domain/port"
)

type Registry struct{}

var _ port.TargetRegistry = (*Registry)(nil)

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) EnabledTargets() ([]models.BackupTarget, error) {
	if err := validateRemoteConfig(); err != nil {
		return nil, err
	}

	ids, err := parseEnabledTargets(config.App.EnabledTargets)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, models.ErrNoEnabledTargets
	}

	targets := make([]models.BackupTarget, 0, len(ids))
	for _, id := range ids {
		target, resolveErr := resolveTarget(id)
		if resolveErr != nil {
			return nil, resolveErr
		}
		targets = append(targets, target)
	}

	return targets, nil
}

func validateRemoteConfig() error {
	if strings.TrimSpace(config.App.RemotePrefix) == "" {
		return models.ErrMissingRemotePrefix
	}
	return nil
}

func parseEnabledTargets(raw string) ([]models.AppID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	ids := make([]models.AppID, 0, len(parts))
	seen := make(map[models.AppID]struct{}, len(parts))

	for _, part := range parts {
		id := models.AppID(strings.TrimSpace(strings.ToLower(part)))
		if id == "" {
			continue
		}
		if !isKnownApp(id) {
			return nil, models.ErrUnknownTarget
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	return ids, nil
}

func isKnownApp(id models.AppID) bool {
	switch id {
	case models.AppSonarr, models.AppRadarr, models.AppProwlarr, models.AppProfilarr,
		models.AppLidarr, models.AppBazarr, models.AppNavidrome, models.AppSeerr,
		models.AppSoulsync, models.AppKomodo, models.AppPostgres:
		return true
	default:
		return false
	}
}

func resolveTarget(id models.AppID) (models.BackupTarget, error) {
	if id == models.AppKomodo {
		if err := validateKomodoMongo(); err != nil {
			return models.BackupTarget{}, err
		}
		return models.BackupTarget{
			ID:           id,
			Kind:         models.KindMongoDump,
			RemoteSubdir: string(id),
		}, nil
	}

	if id == models.AppPostgres {
		if err := validatePostgres(); err != nil {
			return models.BackupTarget{}, err
		}
		return models.BackupTarget{
			ID:           id,
			Kind:         models.KindPostgresDump,
			RemoteSubdir: string(id),
		}, nil
	}

	if apiTarget, ok, err := resolveAPITarget(id); err != nil {
		return models.BackupTarget{}, err
	} else if ok {
		return apiTarget, nil
	}

	path, err := configPathFor(id)
	if err != nil {
		return models.BackupTarget{}, err
	}

	return models.BackupTarget{
		ID:           id,
		Kind:         models.KindConfigSync,
		LocalPath:    path,
		RemoteSubdir: string(id),
	}, nil
}

func resolveAPITarget(id models.AppID) (models.BackupTarget, bool, error) {
	creds := apiCredentials(id)
	if creds.baseURL == "" && creds.apiKey == "" && creds.backupMount == "" {
		return models.BackupTarget{}, false, nil
	}

	creds.apiKey = strings.TrimSpace(creds.apiKey)
	if creds.apiKey == "" && strings.TrimSpace(creds.backupMount) != "" {
		key, err := apiKeyFromBackupMount(id, creds.backupMount)
		if err != nil {
			return models.BackupTarget{}, false, err
		}
		creds.apiKey = key
	}

	if creds.baseURL == "" || creds.apiKey == "" {
		return models.BackupTarget{}, false, models.ErrMissingAPI
	}
	if strings.TrimSpace(creds.backupMount) == "" {
		return models.BackupTarget{}, false, models.ErrMissingBackupMount
	}

	scheme, ok := apiSchemeFor(id)
	if !ok {
		return models.BackupTarget{}, false, nil
	}

	return models.BackupTarget{
		ID:           id,
		Kind:         models.KindArrAPI,
		RemoteSubdir: string(id),
		APIBaseURL:   creds.baseURL,
		APIKey:       creds.apiKey,
		BackupMount:  creds.backupMount,
		APIScheme:    scheme,
	}, true, nil
}

type apiCreds struct {
	baseURL     string
	apiKey      string
	backupMount string
}

func apiCredentials(id models.AppID) apiCreds {
	switch id {
	case models.AppSonarr:
		return apiCreds{config.App.SonarrURL, config.App.SonarrAPIKey, config.App.SonarrBackupMount}
	case models.AppRadarr:
		return apiCreds{config.App.RadarrURL, config.App.RadarrAPIKey, config.App.RadarrBackupMount}
	case models.AppProwlarr:
		return apiCreds{config.App.ProwlarrURL, config.App.ProwlarrAPIKey, config.App.ProwlarrBackupMount}
	case models.AppLidarr:
		return apiCreds{config.App.LidarrURL, config.App.LidarrAPIKey, config.App.LidarrBackupMount}
	case models.AppBazarr:
		return apiCreds{config.App.BazarrURL, config.App.BazarrAPIKey, config.App.BazarrBackupMount}
	default:
		return apiCreds{}
	}
}

func apiSchemeFor(id models.AppID) (models.APIScheme, bool) {
	switch id {
	case models.AppSonarr, models.AppRadarr:
		return models.APISchemeServarrV3, true
	case models.AppProwlarr, models.AppLidarr:
		return models.APISchemeServarrV1, true
	case models.AppBazarr:
		return models.APISchemeBazarr, true
	default:
		return "", false
	}
}

func configPathFor(id models.AppID) (string, error) {
	var path string
	switch id {
	case models.AppSonarr:
		path = config.App.SonarrConfigPath
	case models.AppRadarr:
		path = config.App.RadarrConfigPath
	case models.AppProwlarr:
		path = config.App.ProwlarrConfigPath
	case models.AppProfilarr:
		path = config.App.ProfilarrConfigPath
	case models.AppLidarr:
		path = config.App.LidarrConfigPath
	case models.AppBazarr:
		path = config.App.BazarrConfigPath
	case models.AppNavidrome:
		path = config.App.NavidromeConfigPath
	case models.AppSeerr:
		path = config.App.SeerrConfigPath
	case models.AppSoulsync:
		path = config.App.SoulsyncConfigPath
	default:
		return "", models.ErrUnknownTarget
	}

	path = strings.TrimSpace(path)
	if path == "" {
		return "", models.ErrMissingPath
	}
	if _, err := os.Stat(path); err != nil {
		return "", err
	}

	return path, nil
}

func apiKeyFromBackupMount(id models.AppID, backupMount string) (string, error) {
	switch id {
	case models.AppSonarr, models.AppRadarr, models.AppProwlarr, models.AppLidarr:
		return credentials.ServarrAPIKey(backupMount)
	case models.AppBazarr:
		return credentials.BazarrAPIKey(backupMount)
	default:
		return "", models.ErrMissingAPI
	}
}

func validateKomodoMongo() error {
	if strings.TrimSpace(config.App.KomodoMongoURI) != "" {
		return nil
	}
	if strings.TrimSpace(config.App.KomodoMongoAddress) == "" {
		return models.ErrMissingMongo
	}
	return nil
}

func validatePostgres() error {
	if strings.TrimSpace(config.App.PostgresURI) != "" {
		return nil
	}
	if strings.TrimSpace(config.App.PostgresHost) == "" {
		return models.ErrMissingPostgres
	}
	if !config.App.PostgresAllDatabases && strings.TrimSpace(config.App.PostgresDBName) == "" {
		return models.ErrMissingPostgres
	}
	return nil
}
