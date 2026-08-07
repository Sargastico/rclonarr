package service

import (
	"testing"
	"time"

	"github.com/Sargastico/rclonarr/internal/core/domain/port"
	"github.com/stretchr/testify/assert"
)

func TestSelectExpiredBackups(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	retention := 30 * 24 * time.Hour

	tests := []struct {
		name  string
		files []port.RemoteFile
		want  []string
	}{
		{
			name:  "empty",
			files: nil,
			want:  nil,
		},
		{
			name: "all within window",
			files: []port.RemoteFile{
				{Name: "sonarr_backup_20260801_030000.zip", Path: "/r/sonarr_backup_20260801_030000.zip"},
				{Name: "sonarr_backup_20260805_030000.zip", Path: "/r/sonarr_backup_20260805_030000.zip"},
			},
			want: nil,
		},
		{
			name: "drop older when window has entries",
			files: []port.RemoteFile{
				{Name: "sonarr_backup_20260601_030000.zip", Path: "/r/sonarr_backup_20260601_030000.zip"},
				{Name: "sonarr_backup_20260801_030000.zip", Path: "/r/sonarr_backup_20260801_030000.zip"},
			},
			want: []string{"/r/sonarr_backup_20260601_030000.zip"},
		},
		{
			name: "keep all when nothing in window",
			files: []port.RemoteFile{
				{Name: "sonarr_backup_20260501_030000.zip", Path: "/r/sonarr_backup_20260501_030000.zip"},
				{Name: "sonarr_backup_20260601_030000.zip", Path: "/r/sonarr_backup_20260601_030000.zip"},
			},
			want: nil,
		},
		{
			name: "ignore non versioned names",
			files: []port.RemoteFile{
				{Name: "readme.txt", Path: "/r/readme.txt"},
				{Name: "sonarr_backup_20260601_030000.zip", Path: "/r/sonarr_backup_20260601_030000.zip"},
				{Name: "sonarr_backup_20260801_030000.zip", Path: "/r/sonarr_backup_20260801_030000.zip"},
			},
			want: []string{"/r/sonarr_backup_20260601_030000.zip"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SelectExpiredBackups(tt.files, now, retention)
			assert.Equal(t, tt.want, got)
		})
	}
}
