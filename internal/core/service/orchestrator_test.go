package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Sargastico/rclonarr/internal/core/domain/models"
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

type mockSyncer struct {
	mock.Mock
}

func (m *mockSyncer) Sync(ctx context.Context, localPath, remotePath string) error {
	args := m.Called(ctx, localPath, remotePath)
	return args.Error(0)
}

func (m *mockSyncer) Copy(ctx context.Context, localPath, remotePath string) error {
	args := m.Called(ctx, localPath, remotePath)
	return args.Error(0)
}

func (m *mockSyncer) RemotePath(subdir string) string {
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
	syncer := new(mockSyncer)
	dumper := new(mockDumper)
	arr := new(mockArrTrigger)

	targets := []models.BackupTarget{{
		ID:           models.AppSonarr,
		Kind:         models.KindConfigSync,
		LocalPath:    "/data/sonarr",
		RemoteSubdir: "sonarr",
	}}

	reg.On("EnabledTargets").Return(targets, nil)
	syncer.On("RemotePath", "sonarr").Return("b2:homelab/sonarr")
	syncer.On("Sync", mock.Anything, "/data/sonarr", "b2:homelab/sonarr").Return(nil)

	orch := NewOrchestrator(reg, syncer, dumper, arr, syncer)
	results, err := orch.Run(context.Background())

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Succeeded())
}

func TestOrchestrator_Run_ArrAPI(t *testing.T) {
	t.Parallel()

	reg := new(mockRegistry)
	syncer := new(mockSyncer)
	dumper := new(mockDumper)
	arr := new(mockArrTrigger)

	targets := []models.BackupTarget{{
		ID:           models.AppSonarr,
		Kind:         models.KindArrAPI,
		RemoteSubdir: "sonarr",
		APIScheme:    models.APISchemeServarrV3,
	}}

	reg.On("EnabledTargets").Return(targets, nil)
	arr.On("Trigger", mock.Anything, targets[0]).Return("/mount/Backups/sonarr.zip", nil)
	syncer.On("RemotePath", "sonarr").Return("b2:homelab/sonarr")
	syncer.On("Copy", mock.Anything, "/mount/Backups/sonarr.zip", "b2:homelab/sonarr/sonarr.zip").Return(nil)

	orch := NewOrchestrator(reg, syncer, dumper, arr, syncer)
	results, err := orch.Run(context.Background())

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Succeeded())
}

func TestOrchestrator_Run_PartialFailure(t *testing.T) {
	t.Parallel()

	reg := new(mockRegistry)
	syncer := new(mockSyncer)
	dumper := new(mockDumper)
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

	syncErr := errors.New("sync failed")

	reg.On("EnabledTargets").Return(targets, nil)
	syncer.On("RemotePath", "sonarr").Return("b2:homelab/sonarr")
	syncer.On("RemotePath", "radarr").Return("b2:homelab/radarr")
	syncer.On("Sync", mock.Anything, "/data/sonarr", "b2:homelab/sonarr").Return(nil)
	syncer.On("Sync", mock.Anything, "/data/radarr", "b2:homelab/radarr").Return(syncErr)

	orch := NewOrchestrator(reg, syncer, dumper, arr, syncer)
	results, err := orch.Run(context.Background())

	require.Error(t, err)
	require.Len(t, results, 2)
	assert.True(t, results[0].Succeeded())
	assert.False(t, results[1].Succeeded())
}

func TestOrchestrator_Run_Komodo(t *testing.T) {
	t.Parallel()

	reg := new(mockRegistry)
	syncer := new(mockSyncer)
	dumper := new(mockDumper)
	arr := new(mockArrTrigger)

	targets := []models.BackupTarget{{
		ID:           models.AppKomodo,
		Kind:         models.KindMongoDump,
		RemoteSubdir: "komodo",
	}}

	reg.On("EnabledTargets").Return(targets, nil)
	dumper.On("Dump", mock.Anything, mock.AnythingOfType("string")).Return(nil)
	syncer.On("RemotePath", "komodo").Return("b2:homelab/komodo")
	syncer.On("Sync", mock.Anything, mock.AnythingOfType("string"), "b2:homelab/komodo").Return(nil)

	orch := NewOrchestrator(reg, syncer, dumper, arr, syncer)
	results, err := orch.Run(context.Background())

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Succeeded())
}

func TestHasFailures(t *testing.T) {
	t.Parallel()

	assert.False(t, HasFailures([]models.BackupResult{{Err: nil}}))
	assert.True(t, HasFailures([]models.BackupResult{{Err: errors.New("x")}}))
}
