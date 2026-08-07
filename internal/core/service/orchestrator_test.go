package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/Sargastico/rclonarr/internal/core/domain/models"
	"github.com/Sargastico/rclonarr/internal/core/domain/port"
	"github.com/Sargastico/rclonarr/internal/platform/archive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRegistry struct {
	mock.Mock
}

func (m *mockRegistry) EnabledTargets() ([]models.BackupTarget, error) {
	args := m.Called()
	targets, _ := args.Get(0).([]models.BackupTarget)
	return targets, args.Error(1)
}

type mockUploader struct {
	mock.Mock
}

func (m *mockUploader) Upload(ctx context.Context, localPath string, remoteDir string) error {
	args := m.Called(ctx, localPath, remoteDir)
	return args.Error(0)
}

func (m *mockUploader) EnsureAuth(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockUploader) ListFiles(ctx context.Context, remoteDir string) ([]port.RemoteFile, error) {
	args := m.Called(ctx, remoteDir)
	files, _ := args.Get(0).([]port.RemoteFile)
	return files, args.Error(1)
}

func (m *mockUploader) Trash(ctx context.Context, remotePaths ...string) error {
	args := m.Called(ctx, remotePaths)
	return args.Error(0)
}

func (m *mockUploader) RemotePath(subdir string) string {
	args := m.Called(subdir)
	return args.String(0)
}

type mockDumper struct {
	mock.Mock
}

func (m *mockDumper) Dump(ctx context.Context, dumpDir string) error {
	args := m.Called(ctx, dumpDir)
	return args.Error(0)
}

type mockPgDumper struct {
	mock.Mock
}

func (m *mockPgDumper) Dump(ctx context.Context, dumpFile string) error {
	args := m.Called(ctx, dumpFile)
	return args.Error(0)
}

type mockArrTrigger struct {
	mock.Mock
}

func (m *mockArrTrigger) Trigger(ctx context.Context, target models.BackupTarget) (string, error) {
	args := m.Called(ctx, target)
	return args.String(0), args.Error(1)
}

func TestOrchestrator_Run_ConfigSync(t *testing.T) {
	t.Parallel()

	reg := new(mockRegistry)
	uploader := new(mockUploader)
	dumper := new(mockDumper)
	pgDumper := new(mockPgDumper)
	arr := new(mockArrTrigger)

	targets := []models.BackupTarget{{
		ID:           models.AppSonarr,
		Kind:         models.KindConfigSync,
		LocalPath:    "/data/sonarr",
		RemoteSubdir: "sonarr",
	}}

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.xml"), []byte("x"), 0o644))

	targets[0].LocalPath = dir

	reg.On("EnabledTargets").Return(targets, nil)
	uploader.On("EnsureAuth", mock.Anything).Return(nil)
	uploader.On("RemotePath", "sonarr").Return("/my-files/homelab-backups/sonarr")
	uploader.On("Upload", mock.Anything, mock.AnythingOfType("string"), "/my-files/homelab-backups/sonarr").Return(nil)

	orch := NewOrchestrator(reg, uploader, dumper, pgDumper, arr, uploader)
	results, err := orch.Run(context.Background())

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Succeeded())
}

func TestOrchestrator_Run_ArrAPI(t *testing.T) {
	t.Parallel()

	reg := new(mockRegistry)
	uploader := new(mockUploader)
	dumper := new(mockDumper)
	pgDumper := new(mockPgDumper)
	arr := new(mockArrTrigger)

	targets := []models.BackupTarget{{
		ID:           models.AppSonarr,
		Kind:         models.KindArrAPI,
		RemoteSubdir: "sonarr",
		APIScheme:    models.APISchemeServarrV3,
	}}

	reg.On("EnabledTargets").Return(targets, nil)
	uploader.On("EnsureAuth", mock.Anything).Return(nil)
	arr.On("Trigger", mock.Anything, targets[0]).Return("/mount/Backups/sonarr.zip", nil)
	uploader.On("RemotePath", "sonarr").Return("/my-files/homelab-backups/sonarr")
	uploader.On("Upload", mock.Anything, "/mount/Backups/sonarr.zip", "/my-files/homelab-backups/sonarr").Return(nil)

	orch := NewOrchestrator(reg, uploader, dumper, pgDumper, arr, uploader)
	results, err := orch.Run(context.Background())

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Succeeded())
}

func TestOrchestrator_Run_PartialFailure(t *testing.T) {
	t.Parallel()

	reg := new(mockRegistry)
	uploader := new(mockUploader)
	dumper := new(mockDumper)
	pgDumper := new(mockPgDumper)
	arr := new(mockArrTrigger)

	targets := []models.BackupTarget{
		{
			ID:           models.AppSonarr,
			Kind:         models.KindConfigSync,
			LocalPath:    "/data/sonarr",
			RemoteSubdir: "sonarr",
		},
		{
			ID:           models.AppRadarr,
			Kind:         models.KindConfigSync,
			LocalPath:    "/data/radarr",
			RemoteSubdir: "radarr",
		},
	}

	syncErr := errors.New("upload failed")

	reg.On("EnabledTargets").Return(targets, nil)
	sonarrDir := t.TempDir()
	radarrDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sonarrDir, "config.xml"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(radarrDir, "config.xml"), []byte("x"), 0o644))
	targets[0].LocalPath = sonarrDir
	targets[1].LocalPath = radarrDir

	uploader.On("EnsureAuth", mock.Anything).Return(nil)
	uploader.On("RemotePath", "sonarr").Return("/my-files/homelab-backups/sonarr")
	uploader.On("RemotePath", "radarr").Return("/my-files/homelab-backups/radarr")
	uploader.On("Upload", mock.Anything, mock.AnythingOfType("string"), "/my-files/homelab-backups/sonarr").Return(nil)
	uploader.On("Upload", mock.Anything, mock.AnythingOfType("string"), "/my-files/homelab-backups/radarr").Return(syncErr)

	orch := NewOrchestrator(reg, uploader, dumper, pgDumper, arr, uploader)
	results, err := orch.Run(context.Background())

	require.Error(t, err)
	require.Len(t, results, 2)
	assert.True(t, results[0].Succeeded())
	assert.False(t, results[1].Succeeded())
}

func TestOrchestrator_Run_Komodo(t *testing.T) {
	t.Parallel()

	reg := new(mockRegistry)
	uploader := new(mockUploader)
	dumper := new(mockDumper)
	pgDumper := new(mockPgDumper)
	arr := new(mockArrTrigger)

	targets := []models.BackupTarget{{
		ID:           models.AppKomodo,
		Kind:         models.KindMongoDump,
		RemoteSubdir: "komodo",
	}}

	reg.On("EnabledTargets").Return(targets, nil)
	uploader.On("EnsureAuth", mock.Anything).Return(nil)
	dumper.On("Dump", mock.Anything, mock.AnythingOfType("string")).Run(func(args mock.Arguments) {
		dumpDir := args.String(1)
		require.NoError(t, os.WriteFile(filepath.Join(dumpDir, "komodo.bson"), []byte("dump"), 0o644))
	}).Return(nil)
	uploader.On("RemotePath", "komodo").Return("/my-files/homelab-backups/komodo")
	uploader.On("Upload", mock.Anything, mock.AnythingOfType("string"), "/my-files/homelab-backups/komodo").Return(nil)

	orch := NewOrchestrator(reg, uploader, dumper, pgDumper, arr, uploader)
	results, err := orch.Run(context.Background())

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Succeeded())
}

func TestOrchestrator_Run_Postgres(t *testing.T) {
	t.Parallel()

	reg := new(mockRegistry)
	uploader := new(mockUploader)
	dumper := new(mockDumper)
	pgDumper := new(mockPgDumper)
	arr := new(mockArrTrigger)

	targets := []models.BackupTarget{{
		ID:           models.AppPostgres,
		Kind:         models.KindPostgresDump,
		RemoteSubdir: "postgres",
	}}

	reg.On("EnabledTargets").Return(targets, nil)
	uploader.On("EnsureAuth", mock.Anything).Return(nil)
	pgDumper.On("Dump", mock.Anything, mock.MatchedBy(func(path string) bool {
		base := filepath.Base(path)
		return strings.HasPrefix(base, "postgres_backup_") && strings.HasSuffix(base, ".dump")
	})).Run(func(args mock.Arguments) {
		require.NoError(t, os.WriteFile(args.String(1), []byte("PGCUSTOM"), 0o644))
	}).Return(nil)
	uploader.On("RemotePath", "postgres").Return("/my-files/homelab-backups/postgres")
	uploader.On("Upload", mock.Anything, mock.MatchedBy(func(path string) bool {
		return strings.HasSuffix(path, ".dump")
	}), "/my-files/homelab-backups/postgres").Return(nil)

	orch := NewOrchestrator(reg, uploader, dumper, pgDumper, arr, uploader)
	results, err := orch.Run(context.Background())

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Succeeded())
}

func TestOrchestrator_Run_PostgresAllDatabases(t *testing.T) {
	// Not parallel: mutates config.App
	prev := config.App.PostgresAllDatabases
	config.App.PostgresAllDatabases = true
	t.Cleanup(func() { config.App.PostgresAllDatabases = prev })

	reg := new(mockRegistry)
	uploader := new(mockUploader)
	dumper := new(mockDumper)
	pgDumper := new(mockPgDumper)
	arr := new(mockArrTrigger)

	targets := []models.BackupTarget{{
		ID:           models.AppPostgres,
		Kind:         models.KindPostgresDump,
		RemoteSubdir: "postgres",
	}}

	reg.On("EnabledTargets").Return(targets, nil)
	uploader.On("EnsureAuth", mock.Anything).Return(nil)
	pgDumper.On("Dump", mock.Anything, mock.AnythingOfType("string")).Run(func(args mock.Arguments) {
		dir := args.String(1)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "miniflux.dump"), []byte("x"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "kan.dump"), []byte("y"), 0o644))
	}).Return(nil)
	uploader.On("RemotePath", "postgres").Return("/my-files/homelab-backups/postgres")
	uploader.On("Upload", mock.Anything, mock.MatchedBy(func(path string) bool {
		return strings.HasSuffix(path, ".zip")
	}), "/my-files/homelab-backups/postgres").Return(nil)

	orch := NewOrchestrator(reg, uploader, dumper, pgDumper, arr, uploader)
	results, err := orch.Run(context.Background())

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Succeeded())
}

func TestOrchestrator_Run_RetentionCleanup(t *testing.T) {
	prev := config.App.RetentionDays
	config.App.RetentionDays = 30
	t.Cleanup(func() { config.App.RetentionDays = prev })

	reg := new(mockRegistry)
	uploader := new(mockUploader)
	dumper := new(mockDumper)
	pgDumper := new(mockPgDumper)
	arr := new(mockArrTrigger)

	targets := []models.BackupTarget{{
		ID:           models.AppSonarr,
		Kind:         models.KindConfigSync,
		RemoteSubdir: "sonarr",
	}}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.xml"), []byte("x"), 0o644))
	targets[0].LocalPath = dir

	oldName := archive.VersionedZipName("sonarr", time.Now().UTC().AddDate(0, 0, -60))
	freshName := archive.VersionedZipName("sonarr", time.Now().UTC().AddDate(0, 0, -2))
	oldPath := "/my-files/homelab-backups/sonarr/" + oldName
	freshPath := "/my-files/homelab-backups/sonarr/" + freshName

	reg.On("EnabledTargets").Return(targets, nil)
	uploader.On("EnsureAuth", mock.Anything).Return(nil)
	uploader.On("RemotePath", "sonarr").Return("/my-files/homelab-backups/sonarr")
	uploader.On("Upload", mock.Anything, mock.AnythingOfType("string"), "/my-files/homelab-backups/sonarr").Return(nil)
	uploader.On("ListFiles", mock.Anything, "/my-files/homelab-backups/sonarr").Return([]port.RemoteFile{
		{Name: oldName, Path: oldPath},
		{Name: freshName, Path: freshPath},
	}, nil)
	uploader.On("Trash", mock.Anything, []string{oldPath}).Return(nil)

	orch := NewOrchestrator(reg, uploader, dumper, pgDumper, arr, uploader)
	results, err := orch.Run(context.Background())

	require.NoError(t, err)
	require.Len(t, results, 1)
	uploader.AssertExpectations(t)
}

func TestOrchestrator_Run_AuthFailure(t *testing.T) {
	t.Parallel()

	reg := new(mockRegistry)
	uploader := new(mockUploader)
	dumper := new(mockDumper)
	pgDumper := new(mockPgDumper)
	arr := new(mockArrTrigger)

	uploader.On("EnsureAuth", mock.Anything).Return(errors.New("need login"))

	orch := NewOrchestrator(reg, uploader, dumper, pgDumper, arr, uploader)
	results, err := orch.Run(context.Background())

	require.Error(t, err)
	assert.Nil(t, results)
	reg.AssertNotCalled(t, "EnabledTargets")
}

func TestHasFailures(t *testing.T) {
	t.Parallel()

	assert.False(t, HasFailures([]models.BackupResult{{Err: nil}}))
	assert.True(t, HasFailures([]models.BackupResult{{Err: errors.New("x")}}))
}
