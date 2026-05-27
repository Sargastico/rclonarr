package service

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	syncer     port.RcloneSyncer
	dumper     port.MongoDumper
	arrTrigger port.ArrBackupTrigger
	remote     remotePathBuilder
}

type remotePathBuilder interface {
	RemotePath(subdir string) string
}

func NewOrchestrator(
	registry port.TargetRegistry,
	syncer port.RcloneSyncer,
	dumper port.MongoDumper,
	arrTrigger port.ArrBackupTrigger,
	remote remotePathBuilder,
) *Orchestrator {
	return &Orchestrator{
		registry:   registry,
		syncer:     syncer,
		dumper:     dumper,
		arrTrigger: arrTrigger,
		remote:     remote,
	}
}

var _ port.BackupRunner = (*Orchestrator)(nil)

func (o *Orchestrator) Run(ctx context.Context) ([]models.BackupResult, error) {
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

	if failed > 0 {
		return results, fmt.Errorf("%d of %d backup targets failed", failed, len(targets))
	}

	return results, nil
}

func (o *Orchestrator) backupConfig(ctx context.Context, target models.BackupTarget) error {
	at := time.Now().UTC()
	zipPath, cleanup, err := archive.ZipDirectoryToTemp(target.LocalPath, string(target.ID), at)
	if err != nil {
		return fmt.Errorf("zip config for %s: %w", target.ID, err)
	}
	defer cleanup()

	return o.copyVersionedZip(ctx, target.RemoteSubdir, zipPath, at)
}

func (o *Orchestrator) backupArrAPI(ctx context.Context, target models.BackupTarget) error {
	localFile, err := o.arrTrigger.Trigger(ctx, target)
	if err != nil {
		return err
	}
	if !archive.IsZipFile(localFile) {
		return fmt.Errorf("%s api backup is not a .zip file: %q", target.ID, localFile)
	}

	return o.copyVersionedZip(ctx, target.RemoteSubdir, localFile, time.Now().UTC())
}

func (o *Orchestrator) backupKomodo(ctx context.Context, target models.BackupTarget) error {
	dumpDir, err := resolveDumpDir()
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

	return o.copyVersionedZip(ctx, target.RemoteSubdir, zipPath, at)
}

func (o *Orchestrator) copyVersionedZip(ctx context.Context, remoteSubdir, localZip string, at time.Time) error {
	name := archive.VersionedZipName(remoteSubdir, at)
	remoteFile := o.remote.RemotePath(remoteSubdir) + "/" + name

	otelzap.L().InfoContext(ctx, "uploading versioned backup",
		zap.String("local_zip", localZip),
		zap.String("remote_zip", name),
	)

	if err := o.syncer.Copy(ctx, localZip, remoteFile); err != nil {
		return fmt.Errorf("copy versioned backup %q: %w", name, err)
	}

	return nil
}

func resolveDumpDir() (string, error) {
	dir := config.App.KomodoDumpDir
	if dir == "" {
		return "", nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("komodo dump dir is not a directory: %s", dir)
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
