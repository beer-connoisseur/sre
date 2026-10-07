package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"urlshort/generated/api"
	"urlshort/internal/config"
	"urlshort/internal/controller"
	"urlshort/internal/controller/middleware"
	"urlshort/internal/repository/postgres"
	"urlshort/internal/usecase/link"
	"urlshort/migrations"
)

const readHeaderTimeout = 5 * time.Second

func Serve(ctx context.Context, logger *zap.Logger, cfg *config.Config) error {
	pool, err := pgxpool.New(ctx, cfg.PostgresURL())
	if err != nil {
		return err
	}
	defer pool.Close()

	linkUseCase := link.NewService(postgres.NewLinkRepository(pool))
	ctrl := controller.New(logger, linkUseCase, pool)

	validator, err := middleware.OapiValidator()
	if err != nil {
		return err
	}

	router := gin.New()
	router.Use(
		ginzap.GinzapWithConfig(logger, &ginzap.Config{
			UTC:          true,
			DefaultLevel: zapcore.InfoLevel,
			SkipPaths:    []string{"/healthz", "/readyz"},
		}),
		ginzap.RecoveryWithZap(logger, true),
		validator,
	)
	api.RegisterHandlers(router, ctrl)

	server := &http.Server{
		Addr:              net.JoinHostPort("", cfg.HTTPPort),
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", zap.String("addr", server.Addr))
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutdown signal received, draining connections", zap.Duration("timeout", cfg.ShutdownTimeout))

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	logger.Info("http server stopped")

	return nil
}

func Migrate(ctx context.Context, logger *zap.Logger, cfg *config.Config) error {
	pool, err := pgxpool.New(ctx, cfg.PostgresURL())
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := migrations.Up(ctx, pool); err != nil {
		return err
	}

	logger.Info("migrations applied")

	return nil
}
