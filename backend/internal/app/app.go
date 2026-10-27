package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/IBM/pgxpoolprometheus"
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
	"urlshort/internal/prober"
	"urlshort/internal/repository/postgres"
	"urlshort/internal/usecase/link"
	"urlshort/migrations"
)

const (
	readHeaderTimeout = 5 * time.Second
	proberUserAgent   = "urlshort-prober"
)

func Serve(ctx context.Context, logger *zap.Logger, cfg *config.Config) error {
	pool, err := pgxpool.New(ctx, cfg.PostgresURL())
	if err != nil {
		return err
	}
	defer pool.Close()

	registry := metrics.NewRegistry(pgxpoolprometheus.NewCollector(pool, map[string]string{"db_name": cfg.PG.DB}))

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
		middleware.Chaos(cfg.Chaos.ErrorRate, cfg.Chaos.Latency, "/healthz", "/readyz"),
		validator,
	)
	api.RegisterHandlers(router, ctrl)

	if cfg.Chaos.ErrorRate > 0 || cfg.Chaos.Latency > 0 {
		logger.Warn("chaos enabled", zap.Float64("error_rate", cfg.Chaos.ErrorRate), zap.Duration("latency", cfg.Chaos.Latency))
	}

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

	return serve(ctx, logger, cfg.ShutdownTimeout, map[string]*http.Server{"api": apiServer, "metrics": metricsServer})
}

func Probe(ctx context.Context, logger *zap.Logger, cfg *config.Prober) error {
	registry := metrics.NewRegistry()

	p, err := prober.New(cfg.TargetURL, proberUserAgent, cfg.Timeout, logger, registry)
	if err != nil {
		return err
	}

	metricsServer := &http.Server{
		Addr:              net.JoinHostPort("", cfg.MetricsPort),
		Handler:           metrics.Handler(registry),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	logger.Info("prober started", zap.String("target", cfg.TargetURL), zap.Duration("interval", cfg.Interval))

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		p.Run(groupCtx, cfg.Interval)
		return nil
	})
	group.Go(func() error {
		return serve(groupCtx, logger, cfg.ShutdownTimeout, map[string]*http.Server{"metrics": metricsServer})
	})

	return group.Wait()
}

func serve(ctx context.Context, logger *zap.Logger, shutdownTimeout time.Duration, servers map[string]*http.Server) error {
	group, groupCtx := errgroup.WithContext(ctx)
	for name, server := range servers {
		group.Go(func() error { return listen(logger, name, server) })
	}
	group.Go(func() error {
		<-groupCtx.Done()

		logger.Info("shutting down, draining connections", zap.Duration("timeout", shutdownTimeout))

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		errs := make([]error, 0, len(servers))
		for _, server := range servers {
			errs = append(errs, server.Shutdown(shutdownCtx))
		}
		if err := errors.Join(errs...); err != nil {
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
