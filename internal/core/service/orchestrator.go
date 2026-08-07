package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/Sargastico/rclonarr/internal/core/domain/models"
	"github.com/Sargastico/rclonarr/internal/core/domain/port"
	"github.com/Sargastico/rclonarr/internal/platform/archive"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
)

type Orchestrator struct {
	registry   port.TargetRegistry
	uploader   port.RemoteUploader
	dumper     port.MongoDumper
	pgDumper   port.PostgresDumper
	arrTrigger port.ArrBackupTrigger
	remote     remotePathBuilder
}

type remotePathBuilder interface {
	RemotePath(subdir string) string
}

func NewOrchestrator(
	registry port.TargetRegistry,
	uploader port.RemoteUploader,
	dumper port.MongoDumper,
	pgDumper port.PostgresDumper,
	arrTrigger port.ArrBackupTrigger,
	remote remotePathBuilder,
) *Orchestrator {
	return &Orchestrator{
		registry:   registry,
		uploader:   uploader,
		dumper:     dumper,
		pgDumper:   pgDumper,
		arrTrigger: arrTrigger,
		remote:     remote,
	}
}

var _ port.BackupRunner = (*Orchestrator)(nil)

func (o *Orchestrator) Run(ctx context.Context) ([]models.BackupResult, error) {
	if err := o.uploader.EnsureAuth(ctx); err != nil {
		return nil, fmt.Errorf("proton drive auth: %w", err)
	}

	targets, err := o.registry.EnabledTargets()
	if err != nil {
		return nil, err
	}

	results := make([]models.BackupResult, 0, len(targets))
	var failed int

	for _, target := range targets {
		result := models.BackupResult{Target: target.ID}
		otelzap.L().InfoContext(ctx, "starting backup", zap.String("target", string(target.ID)))

		switch target.Kind {
		case models.KindConfigSync:
			result.Err = o.backupConfig(ctx, target)
		case models.KindArrAPI:
			result.Err = o.backupArrAPI(ctx, target)
		case models.KindMongoDump:
			result.Err = o.backupKomodo(ctx, target)
		case models.KindPostgresDump:
			result.Err = o.backupPostgres(ctx, target)
		default:
			result.Err = fmt.Errorf("unsupported backup kind %q", target.Kind)
		}

		if result.Err != nil {
			failed++
			otelzap.L().ErrorContext(ctx, "backup failed",
				zap.String("target", string(target.ID)),
				zap.Error(result.Err),
			)
		} else {
			otelzap.L().InfoContext(ctx, "backup succeeded", zap.String("target", string(target.ID)))
		}

		results = append(results, result)
	}

	o.cleanupSuccessful(ctx, results)

	if failed > 0 {
		return results, fmt.Errorf("%d of %d backup targets failed", failed, len(targets))
	}

	return results, nil
}

func (o *Orchestrator) cleanupSuccessful(ctx context.Context, results []models.BackupResult) {
	days := config.App.RetentionDays
	if days <= 0 {
		return
	}
	retention := time.Duration(days) * 24 * time.Hour
	now := time.Now().UTC()

	for _, result := range results {
		if result.Err != nil {
			continue
		}
		remoteDir := o.remote.RemotePath(string(result.Target))
		if err := o.cleanupRemoteDir(ctx, remoteDir, now, retention); err != nil {
			otelzap.L().WarnContext(ctx, "backup retention cleanup failed",
				zap.String("target", string(result.Target)),
				zap.String("remote_dir", remoteDir),
				zap.Error(err),
			)
		}
	}
}

func (o *Orchestrator) cleanupRemoteDir(ctx context.Context, remoteDir string, now time.Time, retention time.Duration) error {
	files, err := o.uploader.ListFiles(ctx, remoteDir)
	if err != nil {
		return err
	}

	expired := SelectExpiredBackups(files, now, retention)
	if len(expired) == 0 {
		otelzap.L().InfoContext(ctx, "backup retention: nothing to trash",
			zap.String("remote_dir", remoteDir),
			zap.Int("files", len(files)),
			zap.Int("retention_days", config.App.RetentionDays),
		)
		return nil
	}

	otelzap.L().InfoContext(ctx, "backup retention: trashing old backups",
		zap.String("remote_dir", remoteDir),
		zap.Int("count", len(expired)),
		zap.Strings("paths", expired),
	)

	return o.uploader.Trash(ctx, expired...)
}

func (o *Orchestrator) backupConfig(ctx context.Context, target models.BackupTarget) error {
	at := time.Now().UTC()
	zipPath, cleanup, err := archive.ZipDirectoryToTemp(target.LocalPath, string(target.ID), at)
	if err != nil {
		return fmt.Errorf("zip config for %s: %w", target.ID, err)
	}
	defer cleanup()

	return o.uploadVersioned(ctx, target.RemoteSubdir, zipPath, archive.VersionedZipName(target.RemoteSubdir, at))
}

func (o *Orchestrator) backupArrAPI(ctx context.Context, target models.BackupTarget) error {
	localFile, err := o.arrTrigger.Trigger(ctx, target)
	if err != nil {
		return err
	}
	if !archive.IsZipFile(localFile) {
		return fmt.Errorf("%s api backup is not a .zip file: %q", target.ID, localFile)
	}

	at := time.Now().UTC()
	return o.uploadVersioned(ctx, target.RemoteSubdir, localFile, archive.VersionedZipName(target.RemoteSubdir, at))
}

func (o *Orchestrator) backupKomodo(ctx context.Context, target models.BackupTarget) error {
	dumpDir, err := resolveKomodoDumpDir()
	if err != nil {
		return err
	}

	useTemp := dumpDir == ""
	if useTemp {
		dumpDir, err = os.MkdirTemp("", "rclonarr-komodo-*")
		if err != nil {
			return fmt.Errorf("create komodo dump dir: %w", err)
		}
		defer func() {
			if removeErr := os.RemoveAll(dumpDir); removeErr != nil {
				otelzap.L().WarnContext(ctx, "failed to remove komodo dump dir",
					zap.String("path", dumpDir),
					zap.Error(removeErr),
				)
			}
		}()
	}

	if err := o.dumper.Dump(ctx, dumpDir); err != nil {
		return err
	}

	at := time.Now().UTC()
	zipPath, cleanup, err := archive.ZipDirectoryToTemp(dumpDir, string(target.ID), at)
	if err != nil {
		return fmt.Errorf("zip komodo dump: %w", err)
	}
	defer cleanup()

	return o.uploadVersioned(ctx, target.RemoteSubdir, zipPath, archive.VersionedZipName(target.RemoteSubdir, at))
}

func (o *Orchestrator) backupPostgres(ctx context.Context, target models.BackupTarget) error {
	dumpDir, err := resolvePostgresDumpDir()
	if err != nil {
		return err
	}

	useTemp := dumpDir == ""
	if useTemp {
		dumpDir, err = os.MkdirTemp("", "rclonarr-postgres-*")
		if err != nil {
			return fmt.Errorf("create postgres dump dir: %w", err)
		}
		defer func() {
			if removeErr := os.RemoveAll(dumpDir); removeErr != nil {
				otelzap.L().WarnContext(ctx, "failed to remove postgres dump dir",
					zap.String("path", dumpDir),
					zap.Error(removeErr),
				)
			}
		}()
	}

	at := time.Now().UTC()

	if config.App.PostgresAllDatabases {
		if err := o.pgDumper.Dump(ctx, dumpDir); err != nil {
			return err
		}
		zipPath, cleanup, err := archive.ZipDirectoryToTemp(dumpDir, string(target.ID), at)
		if err != nil {
			return fmt.Errorf("zip postgres dumps: %w", err)
		}
		defer cleanup()
		return o.uploadVersioned(ctx, target.RemoteSubdir, zipPath, archive.VersionedZipName(target.RemoteSubdir, at))
	}

	name := archive.VersionedBackupName(string(target.ID), at, "dump")
	dumpPath := filepath.Join(dumpDir, name)
	if err := o.pgDumper.Dump(ctx, dumpPath); err != nil {
		return err
	}
	return o.uploadVersioned(ctx, target.RemoteSubdir, dumpPath, name)
}

func (o *Orchestrator) uploadVersioned(ctx context.Context, remoteSubdir, localPath, remoteName string) error {
	remoteDir := o.remote.RemotePath(remoteSubdir)

	otelzap.L().InfoContext(ctx, "uploading versioned backup",
		zap.String("local_path", localPath),
		zap.String("remote_dir", remoteDir),
		zap.String("remote_name", remoteName),
	)

	if err := o.uploader.Upload(ctx, localPath, remoteDir); err != nil {
		return fmt.Errorf("upload versioned backup %q: %w", remoteName, err)
	}

	return nil
}

func resolveKomodoDumpDir() (string, error) {
	return resolveConfiguredDumpDir(config.App.KomodoDumpDir, "komodo")
}

func resolvePostgresDumpDir() (string, error) {
	return resolveConfiguredDumpDir(config.App.PostgresDumpDir, "postgres")
}

func resolveConfiguredDumpDir(dir, label string) (string, error) {
	if dir == "" {
		return "", nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s dump dir is not a directory: %s", label, dir)
	}
	return dir, nil
}

// HasFailures reports whether any result failed.
func HasFailures(results []models.BackupResult) bool {
	for _, r := range results {
		if r.Err != nil {
			return true
		}
	}
	return false
}

// FirstError returns the first error in results, if any.
func FirstError(results []models.BackupResult) error {
	for _, r := range results {
		if r.Err != nil {
			return r.Err
		}
	}
	return nil
}

// JoinErrors aggregates result errors.
func JoinErrors(results []models.BackupResult) error {
	var errs []error
	for _, r := range results {
		if r.Err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.Target, r.Err))
		}
	}
	return errors.Join(errs...)
}
