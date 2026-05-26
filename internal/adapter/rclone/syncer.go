package rclone

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	stdsync "sync"
	"strings"

	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/Sargastico/rclonarr/internal/core/domain/port"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/operations"
	fssync "github.com/rclone/rclone/fs/sync"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"

	_ "github.com/rclone/rclone/backend/all"
)

type Syncer struct {
	mu    stdsync.Mutex
	ready bool
}

var _ port.RcloneSyncer = (*Syncer)(nil)

func NewSyncer() *Syncer {
	return &Syncer{}
}

func (s *Syncer) Sync(ctx context.Context, localPath, remotePath string) error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}

	srcFs, err := fs.NewFs(ctx, localPath)
	if err != nil {
		return fmt.Errorf("open local fs %q: %w", localPath, err)
	}

	dstFs, err := fs.NewFs(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("open remote fs %q: %w", remotePath, err)
	}

	otelzap.L().InfoContext(ctx, "starting rclone sync",
		zap.String("source", localPath),
		zap.String("destination", remotePath),
	)

	if err := fssync.Sync(ctx, dstFs, srcFs, true); err != nil {
		return fmt.Errorf("rclone sync %q -> %q: %w", localPath, remotePath, err)
	}

	otelzap.L().InfoContext(ctx, "rclone sync completed",
		zap.String("source", localPath),
		zap.String("destination", remotePath),
	)

	return nil
}

func (s *Syncer) Copy(ctx context.Context, localPath, remotePath string) error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}

	srcDir := filepath.Dir(localPath)
	srcName := filepath.Base(localPath)
	dstDir, dstName := splitRemotePath(remotePath)

	srcFs, err := fs.NewFs(ctx, srcDir)
	if err != nil {
		return fmt.Errorf("open local fs %q: %w", srcDir, err)
	}

	dstFs, err := fs.NewFs(ctx, dstDir)
	if err != nil {
		return fmt.Errorf("open remote fs %q: %w", dstDir, err)
	}

	otelzap.L().InfoContext(ctx, "starting rclone copy",
		zap.String("source", localPath),
		zap.String("destination", remotePath),
	)

	if err := operations.CopyFile(ctx, dstFs, srcFs, dstName, srcName); err != nil {
		return fmt.Errorf("rclone copy %q -> %q: %w", localPath, remotePath, err)
	}

	otelzap.L().InfoContext(ctx, "rclone copy completed",
		zap.String("source", localPath),
		zap.String("destination", remotePath),
	)

	return nil
}

func splitRemotePath(remotePath string) (dir, name string) {
	if idx := strings.LastIndex(remotePath, "/"); idx >= 0 {
		return remotePath[:idx], remotePath[idx+1:]
	}
	return remotePath, ""
}

func (s *Syncer) RemotePath(subdir string) string {
	prefix := config.App.RemotePrefix
	if prefix != "" {
		return fmt.Sprintf("%s:%s/%s", config.App.RemoteName, prefix, subdir)
	}
	return fmt.Sprintf("%s:%s", config.App.RemoteName, subdir)
}

func (s *Syncer) ensureInitialized() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ready {
		return nil
	}

	if config.App.RcloneConfigPath != "" {
		if err := os.Setenv("RCLONE_CONFIG", config.App.RcloneConfigPath); err != nil {
			return fmt.Errorf("set RCLONE_CONFIG: %w", err)
		}
	}

	configfile.Install()

	s.ready = true
	otelzap.L().Info("rclone initialized", zap.String("config", config.App.RcloneConfigPath))

	return nil
}
