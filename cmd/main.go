package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/Sargastico/rclonarr/internal/core/config"
	"github.com/Sargastico/rclonarr/internal/core/service"
	"github.com/Sargastico/rclonarr/internal/di"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
)

func main() {
	os.Exit(run())
}

func run() int {
	if err := initLogger(); err != nil {
		return 1
	}

	if err := loadEnvironment(); err != nil {
		otelzap.L().Fatal("failed to load environment", zap.Error(err))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	container := di.NewContainer(ctx)
	defer func() {
		if err := container.Close(context.Background()); err != nil {
			otelzap.L().Error("failed to close container", zap.Error(err))
		}
	}()

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
		return 1
	}

	otelzap.L().Info("all backups completed successfully")
	return 0
}

func initLogger() error {
	zapLogger, err := zap.NewProduction()
	if err != nil {
		return err
	}

	logger := otelzap.New(zapLogger)
	otelzap.ReplaceGlobals(logger)

	return nil
}

func loadEnvironment() error {
	_ = godotenv.Overload(".env.local")

	if err := envconfig.Process("app", &config.App); err != nil {
		return err
	}

	if config.App.Development {
		otelzap.L().Warn("running in development mode")
	}

	return nil
}
