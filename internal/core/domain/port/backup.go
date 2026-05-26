package port

import (
	"context"

	"github.com/Sargastico/rclonarr/internal/core/domain/models"
)

// BackupRunner runs all configured backup targets once.
type BackupRunner interface {
	Run(ctx context.Context) ([]models.BackupResult, error)
}

// TargetRegistry resolves enabled backup targets from configuration.
type TargetRegistry interface {
	EnabledTargets() ([]models.BackupTarget, error)
}

// RcloneSyncer uploads local paths to a remote rclone destination.
type RcloneSyncer interface {
	Sync(ctx context.Context, localPath, remotePath string) error
	Copy(ctx context.Context, localPath, remotePath string) error
}

// ArrBackupTrigger triggers an in-app backup via HTTP API and returns the local file path.
type ArrBackupTrigger interface {
	Trigger(ctx context.Context, target models.BackupTarget) (localBackupPath string, err error)
}

// MongoDumper dumps a MongoDB database to a local directory.
type MongoDumper interface {
	Dump(ctx context.Context, dumpDir string) error
}
