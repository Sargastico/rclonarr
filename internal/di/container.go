package di

import (
	"context"
	"fmt"
	"time"

	"github.com/Sargastico/rclonarr/internal/adapter/arrbackup"
	"github.com/Sargastico/rclonarr/internal/adapter/mongo"
	"github.com/Sargastico/rclonarr/internal/adapter/postgres"
	"github.com/Sargastico/rclonarr/internal/adapter/protondrive"
	"github.com/Sargastico/rclonarr/internal/adapter/targets"
	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/Sargastico/rclonarr/internal/core/domain/models"
	"github.com/Sargastico/rclonarr/internal/core/domain/port"
	"github.com/Sargastico/rclonarr/internal/core/service"
	"github.com/Sargastico/rclonarr/internal/platform/observability"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const shutdownTimeout = 15 * time.Second

type Container struct {
	runner        port.BackupRunner
	shutdownFuncs []func(context.Context) error
}

func NewContainer(ctx context.Context) *Container {
	var shutdownFuncs []func(context.Context) error

	if config.App.EnableOtel {
		otelShutdown, err := observability.SetupTracing(ctx)
		if err != nil {
			handleFatalError(ctx, "failed to initialize tracing", err)
		}
		if otelShutdown != nil {
			shutdownFuncs = append(shutdownFuncs, otelShutdown)
		}
	}

	if config.App.EnableProfiling {
		otelzap.L().WarnContext(ctx, "profiling is not configured; set APP_ENABLE_PROFILING=false or add a profiler integration")
	}

	uploader := protondrive.NewUploader()
	registry := targets.NewRegistry()
	dumper := mongo.NewDumper()
	pgDumper := postgres.NewDumper()
	arrTrigger := arrbackup.NewTrigger(nil)
	runner := service.NewOrchestrator(registry, uploader, dumper, pgDumper, arrTrigger, uploader)

	return &Container{
		runner:        runner,
		shutdownFuncs: shutdownFuncs,
	}
}

// RunOnce executes all configured backups and returns results.
func (c *Container) RunOnce(ctx context.Context) ([]models.BackupResult, error) {
	if config.App.EnableOtel {
		var span trace.Span
		ctx, span = observability.Tracer().Start(ctx, "rclonarr.RunOnce", trace.WithSpanKind(trace.SpanKindInternal))
		defer span.End()

		results, err := c.runner.Run(ctx)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return results, err
		}

		span.SetStatus(codes.Ok, "backups completed")
		return results, nil
	}

	return c.runner.Run(ctx)
}

func (c *Container) Close(ctx context.Context) error {
	if len(c.shutdownFuncs) == 0 {
		return nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	var errs []error
	for _, fn := range c.shutdownFuncs {
		if err := fn(timeoutCtx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

func handleFatalError(ctx context.Context, message string, err error) {
	otelzap.L().FatalContext(ctx, message, zap.Error(err))
}
