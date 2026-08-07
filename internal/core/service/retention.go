package service

import (
	"path"
	"time"

	"github.com/Sargastico/rclonarr/internal/core/domain/port"
	"github.com/Sargastico/rclonarr/internal/platform/archive"
)

// SelectExpiredBackups returns remote paths that should be trashed.
// Policy: trash backups older than retention, but if no backup falls inside the
// retention window, keep everything (even if older than retention).
func SelectExpiredBackups(files []port.RemoteFile, now time.Time, retention time.Duration) []string {
	if retention <= 0 || len(files) == 0 {
		return nil
	}

	cutoff := now.UTC().Add(-retention)
	var within int
	var expired []string

	for _, f := range files {
		at, ok := backupTime(f)
		if !ok {
			continue // ignore unrecognized names
		}
		if !at.Before(cutoff) {
			within++
			continue
		}
		p := f.Path
		if p == "" {
			p = f.Name
		}
		expired = append(expired, p)
	}

	if within == 0 {
		return nil
	}
	return expired
}

func backupTime(f port.RemoteFile) (time.Time, bool) {
	name := f.Name
	if name == "" {
		name = path.Base(f.Path)
	}
	if at, ok := archive.ParseVersionedBackupTime(name); ok {
		return at, true
	}
	if !f.ModTime.IsZero() {
		return f.ModTime.UTC(), true
	}
	return time.Time{}, false
}
