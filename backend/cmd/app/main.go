package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"urlshort/internal/app"
	"urlshort/internal/config"
)

var version = "dev"

const serviceName = "url-shortener-backend"

type leveled interface {
	Level() string
}

func main() {
	cmd := &cli.Command{
		Name:           "urlshort",
		Usage:          "URL shortener service, configured via environment variables",
		Version:        version,
		DefaultCommand: "serve",
		Commands: []*cli.Command{
			{
				Name:   "serve",
				Usage:  "run HTTP server",
				Action: action(config.New, app.Serve),
			},
			{
				Name:   "migrate",
				Usage:  "apply database migrations and exit",
				Action: action(config.New, app.Migrate),
			},
			{
				Name:   "probe",
				Usage:  "run synthetic prober against the service",
				Action: action(config.NewProber, app.Probe),
			},
		},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := cmd.Run(ctx, os.Args); err != nil {
		cancel()
		os.Exit(1)
	}
}

func action[C leveled](load func() (C, error), run func(context.Context, *zap.Logger, C) error) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) (err error) {
		cfg, err := load()
		if err != nil {
			return cli.Exit(fmt.Sprintf("invalid config: %s", err), 1)
		}

		logger, err := newLogger(cfg.Level())
		if err != nil {
			return cli.Exit(fmt.Sprintf("invalid log level: %s", err), 1)
		}
		defer func() {
			if syncErr := syncLogger(logger); syncErr != nil {
				err = cli.Exit(fmt.Sprintf("sync logger: %s", syncErr), 1)
			}
		}()

		logger = logger.With(
			zap.String("service.name", serviceName),
			zap.String("version", version),
			zap.String("command", cmd.Name),
		)

		if err := run(ctx, logger, cfg); err != nil {
			logger.Error("command failed", zap.Error(err))
			return cli.Exit("", 1)
		}

		return nil
	}
}

func syncLogger(logger *zap.Logger) error {
	err := logger.Sync()
	if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTTY) {
		return nil
	}

	return err
}

func newLogger(level string) (*zap.Logger, error) {
	lvl, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, err
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	cfg.OutputPaths = []string{"stdout"}
	cfg.EncoderConfig.TimeKey = "time"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.DisableStacktrace = true

	return cfg.Build()
}
