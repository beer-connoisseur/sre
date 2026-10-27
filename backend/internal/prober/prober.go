package prober

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"

	"urlshort/generated/api"
)

const (
	stepCreate   = "create"
	stepRedirect = "redirect"
	stepDelete   = "delete"

	resultSuccess = "success"
	resultFailure = "failure"
)

type Prober struct {
	client *api.ClientWithResponses
	logger *zap.Logger

	runs         *prometheus.CounterVec
	stepFailures *prometheus.CounterVec
	stepDuration *prometheus.HistogramVec
	lastSuccess  prometheus.Gauge
}

func New(baseURL, userAgent string, timeout time.Duration, logger *zap.Logger, reg prometheus.Registerer) (*Prober, error) {
	httpClient := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	client, err := api.NewClientWithResponses(baseURL,
		api.WithHTTPClient(httpClient),
		api.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("User-Agent", userAgent)
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}

	factory := promauto.With(reg)
	p := &Prober{
		client: client,
		logger: logger,
		runs: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "urlshort",
			Subsystem: "prober",
			Name:      "runs_total",
			Help:      "Number of prober scenario runs by result.",
		}, []string{"result"}),
		stepFailures: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "urlshort",
			Subsystem: "prober",
			Name:      "step_failures_total",
			Help:      "Number of failed prober steps.",
		}, []string{"step"}),
		stepDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "urlshort",
			Subsystem: "prober",
			Name:      "step_duration_seconds",
			Help:      "Duration of prober steps.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"step"}),
		lastSuccess: factory.NewGauge(prometheus.GaugeOpts{
			Namespace: "urlshort",
			Subsystem: "prober",
			Name:      "last_run_success",
			Help:      "Whether the last prober run succeeded (1) or failed (0).",
		}),
	}

	for _, result := range []string{resultSuccess, resultFailure} {
		p.runs.WithLabelValues(result)
	}
	for _, step := range []string{stepCreate, stepRedirect, stepDelete} {
		p.stepFailures.WithLabelValues(step)
	}

	return p, nil
}

func (p *Prober) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		p.RunOnce(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *Prober) RunOnce(ctx context.Context) {
	if err := p.scenario(ctx); err != nil {
		if ctx.Err() != nil {
			return
		}
		p.runs.WithLabelValues(resultFailure).Inc()
		p.lastSuccess.Set(0)
		p.logger.Warn("probe failed", zap.Error(err))
		return
	}

	p.runs.WithLabelValues(resultSuccess).Inc()
	p.lastSuccess.Set(1)
	p.logger.Debug("probe succeeded")
}

func (p *Prober) scenario(ctx context.Context) error {
	originalURL := "https://example.com/prober/" + strconv.FormatInt(time.Now().UnixNano(), 10)

	var link *api.Link
	err := p.step(stepCreate, func() error {
		resp, err := p.client.CreateLinkWithResponse(ctx, api.CreateLinkJSONRequestBody{OriginalUrl: originalURL})
		if err != nil {
			return err
		}
		if resp.JSON201 == nil {
			return unexpectedStatus(http.StatusCreated, resp.StatusCode())
		}
		link = resp.JSON201
		return nil
	})
	if err != nil {
		return err
	}

	redirectErr := p.step(stepRedirect, func() error {
		resp, err := p.client.RedirectWithResponse(ctx, link.Code)
		if err != nil {
			return err
		}
		if resp.StatusCode() != http.StatusFound {
			return unexpectedStatus(http.StatusFound, resp.StatusCode())
		}
		if location := resp.HTTPResponse.Header.Get("Location"); location != originalURL {
			return fmt.Errorf("unexpected location %q", location)
		}
		return nil
	})

	deleteErr := p.step(stepDelete, func() error {
		resp, err := p.client.DeleteLinkWithResponse(ctx, link.Id)
		if err != nil {
			return err
		}
		if resp.StatusCode() != http.StatusNoContent {
			return unexpectedStatus(http.StatusNoContent, resp.StatusCode())
		}
		return nil
	})

	return errors.Join(redirectErr, deleteErr)
}

func (p *Prober) step(name string, fn func() error) error {
	start := time.Now()
	err := fn()
	p.stepDuration.WithLabelValues(name).Observe(time.Since(start).Seconds())

	if err != nil {
		p.stepFailures.WithLabelValues(name).Inc()
		return fmt.Errorf("%s: %w", name, err)
	}

	return nil
}

func unexpectedStatus(want, got int) error {
	return fmt.Errorf("unexpected status %d, want %d", got, want)
}
