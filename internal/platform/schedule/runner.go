package schedule

import (
	"context"

	"github.com/Sargastico/rclonarr/internal/core/service"
	"github.com/Sargastico/rclonarr/internal/di"
	"github.com/robfig/cron/v3"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
)

// RunDaemon runs a bootstrap backup, then backups on a cron schedule until ctx is cancelled.
func RunDaemon(ctx context.Context, container *di.Container, expr string) error {
	otelzap.L().Info("bootstrap backup starting")
	runBackup(ctx, container, "bootstrap")

	c := cron.New()
	if _, err := c.AddFunc(expr, func() { runBackup(ctx, container, "scheduled") }); err != nil {
		return err
	}

	c.Start()
	defer c.Stop()

	otelzap.L().Info("backup scheduler started", zap.String("schedule", expr))

	<-ctx.Done()
	otelzap.L().Info("backup scheduler stopped")
	return nil
}

func runBackup(ctx context.Context, container *di.Container, phase string) {
	results, err := container.RunOnce(ctx)
	if service.HasFailures(results) {
		for _, r := range results {
			if r.Err != nil {
				otelzap.L().Error("backup target failed",
					zap.String("target", string(r.Target)),
					zap.Error(r.Err),
				)
			}
		}
	}
	if err != nil {
		otelzap.L().Error("backup run failed", zap.Error(err))
		return
	}
	otelzap.L().Info("backup run completed successfully", zap.String("phase", phase))
}
