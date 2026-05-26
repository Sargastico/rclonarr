package arrbackup

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Sargastico/rclonarr/internal/adapter/http/bazarr"
	"github.com/Sargastico/rclonarr/internal/adapter/http/servarr"
	"github.com/Sargastico/rclonarr/internal/core/domain/models"
	"github.com/Sargastico/rclonarr/internal/core/domain/port"
)

// Trigger routes backup requests to the correct *arr HTTP client.
type Trigger struct {
	servarr *servarr.Client
	bazarr  *bazarr.Client
}

var _ port.ArrBackupTrigger = (*Trigger)(nil)

func NewTrigger(httpClient *http.Client) *Trigger {
	return &Trigger{
		servarr: servarr.NewClient(httpClient),
		bazarr:  bazarr.NewClient(httpClient),
	}
}

func (t *Trigger) Trigger(ctx context.Context, target models.BackupTarget) (string, error) {
	switch target.APIScheme {
	case models.APISchemeServarrV1, models.APISchemeServarrV3:
		return t.servarr.Trigger(ctx, target)
	case models.APISchemeBazarr:
		return t.bazarr.Trigger(ctx, target)
	default:
		return "", fmt.Errorf("unsupported api scheme %q", target.APIScheme)
	}
}
