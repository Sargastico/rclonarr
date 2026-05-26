package targets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/Sargastico/rclonarr/internal/core/domain/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEnabledTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    []models.AppID
		wantErr error
	}{
		{
			name: "empty",
			raw:  "",
			want: nil,
		},
		{
			name: "single",
			raw:  "sonarr",
			want: []models.AppID{models.AppSonarr},
		},
		{
			name: "multiple trimmed",
			raw:  " sonarr, radarr ,komodo ",
			want: []models.AppID{models.AppSonarr, models.AppRadarr, models.AppKomodo},
		},
		{
			name:    "unknown",
			raw:     "foo",
			wantErr: models.ErrUnknownTarget,
		},
		{
			name: "dedupe",
			raw:  "sonarr,sonarr",
			want: []models.AppID{models.AppSonarr},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseEnabledTargets(tt.raw)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRegistry_EnabledTargets(t *testing.T) {

	dir := t.TempDir()
	configPath := filepath.Join(dir, "sonarr")
	require.NoError(t, os.Mkdir(configPath, 0o755))

	config.App = config.AppInfo{
		RemoteName:       "b2",
		EnabledTargets:   "sonarr",
		SonarrConfigPath: configPath,
	}

	reg := NewRegistry()
	targets, err := reg.EnabledTargets()
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, models.AppSonarr, targets[0].ID)
	assert.Equal(t, models.KindConfigSync, targets[0].Kind)
	assert.Equal(t, configPath, targets[0].LocalPath)
}

func TestRegistry_ArrAPIRequiresCredentials(t *testing.T) {

	config.App = config.AppInfo{
		RemoteName:     "b2",
		EnabledTargets: "sonarr",
		SonarrURL:      "http://sonarr:8989",
	}

	reg := NewRegistry()
	_, err := reg.EnabledTargets()
	require.ErrorIs(t, err, models.ErrMissingAPI)
}

func TestRegistry_ArrAPIKeyFromConfigXML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.xml"), []byte(`<?xml version="1.0"?>
<Config><ApiKey>from-disk</ApiKey></Config>`), 0o600))

	config.App = config.AppInfo{
		RemoteName:        "b2",
		EnabledTargets:    "sonarr",
		SonarrURL:         "http://sonarr:8989",
		SonarrBackupMount: dir,
	}

	reg := NewRegistry()
	targets, err := reg.EnabledTargets()
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, "from-disk", targets[0].APIKey)
}

func TestRegistry_ArrAPIResolved(t *testing.T) {

	config.App = config.AppInfo{
		RemoteName:        "b2",
		EnabledTargets:    "sonarr",
		SonarrURL:         "http://sonarr:8989",
		SonarrAPIKey:      "secret",
		SonarrBackupMount: "/backup-src/sonarr",
	}

	reg := NewRegistry()
	targets, err := reg.EnabledTargets()
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, models.KindArrAPI, targets[0].Kind)
	assert.Equal(t, models.APISchemeServarrV3, targets[0].APIScheme)
}

func TestRegistry_KomodoRequiresMongo(t *testing.T) {

	config.App = config.AppInfo{
		RemoteName:     "b2",
		EnabledTargets: "komodo",
	}

	reg := NewRegistry()
	_, err := reg.EnabledTargets()
	require.ErrorIs(t, err, models.ErrMissingMongo)
}
