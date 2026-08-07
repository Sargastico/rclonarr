package port

import (
	"context"
	"time"

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

// RemoteFile is a file entry on the remote backup destination.
type RemoteFile struct {
	Name    string
	Path    string
	ModTime time.Time // zero if unknown
}

// RemoteUploader uploads a local file to a remote destination directory.
type RemoteUploader interface {
	Upload(ctx context.Context, localPath, remoteDir string) error
	// EnsureAuth verifies a Proton Drive session and runs auth login if needed
	// (logs the browser URL and blocks until sign-in completes or ctx is cancelled).
	EnsureAuth(ctx context.Context) error
	// ListFiles lists files (not folders) directly under remoteDir.
	ListFiles(ctx context.Context, remoteDir string) ([]RemoteFile, error)
	// Trash moves remote paths to trash (soft delete).
	Trash(ctx context.Context, remotePaths ...string) error
}

// ArrBackupTrigger triggers an in-app backup via HTTP API and returns the local file path.
type ArrBackupTrigger interface {
	Trigger(ctx context.Context, target models.BackupTarget) (localBackupPath string, err error)
}

// MongoDumper dumps a MongoDB database to a local directory.
type MongoDumper interface {
	Dump(ctx context.Context, dumpDir string) error
}

// PostgresDumper dumps PostgreSQL with pg_dump (-Fc).
// dest is a single .dump file path, or a directory when dumping all databases
// (each database written as <name>.dump).
type PostgresDumper interface {
	Dump(ctx context.Context, dest string) error
}
