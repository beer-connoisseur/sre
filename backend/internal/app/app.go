package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/gin-contrib/requestid"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"urlshort/generated/api"
	"urlshort/internal/config"
	"urlshort/internal/controller"
	"urlshort/internal/controller/middleware"
	"urlshort/internal/metrics"
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

	registry := metrics.NewRegistry(pool, cfg.PG.DB)

	linkUseCase := link.NewService(postgres.NewLinkRepository(pool), metrics.NewBusiness(registry))
	ctrl := controller.New(linkUseCase, pool)

	validator, err := middleware.OapiValidator()
	if err != nil {
		return err
	}

	router := gin.New()
	router.Use(
		requestid.New(),
		middleware.Metrics(registry),
		middleware.AccessLog(logger, "/healthz", "/readyz"),
		ginzap.RecoveryWithZap(logger, true),
		validator,
	)
	api.RegisterHandlers(router, ctrl)

	apiServer := &http.Server{
		Addr:              net.JoinHostPort("", cfg.HTTPPort),
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
	}
	metricsServer := &http.Server{
		Addr:              net.JoinHostPort("", cfg.MetricsPort),
		Handler:           metrics.Handler(registry),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error { return listen(logger, "api", apiServer) })
	group.Go(func() error { return listen(logger, "metrics", metricsServer) })
	group.Go(func() error {
		<-groupCtx.Done()

		logger.Info("shutting down, draining connections", zap.Duration("timeout", cfg.ShutdownTimeout))

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := errors.Join(apiServer.Shutdown(shutdownCtx), metricsServer.Shutdown(shutdownCtx)); err != nil {
			return err
		}

		logger.Info("http servers stopped")

		return nil
	})

	return group.Wait()
}

func listen(logger *zap.Logger, name string, server *http.Server) error {
	logger.Info("http server listening", zap.String("server", name), zap.String("addr", server.Addr))

	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

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
