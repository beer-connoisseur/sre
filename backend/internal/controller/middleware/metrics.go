package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const unmatchedRoute = "unmatched"

func Metrics(reg prometheus.Registerer, version string) gin.HandlerFunc {
	factory := promauto.With(reg)

	inFlight := factory.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Number of HTTP requests currently being served.",
	})
	duration := factory.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests by route template and status.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route", "status"})
	requests := factory.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests by version, route template and status.",
	}, []string{"version", "method", "route", "status"})

	return func(c *gin.Context) {
		start := time.Now()
		inFlight.Inc()
		defer inFlight.Dec()

		c.Next()

		route := c.FullPath()
		if route == "" {
			route = unmatchedRoute
		}
		status := strconv.Itoa(c.Writer.Status())

		duration.
			WithLabelValues(c.Request.Method, route, status).
			Observe(time.Since(start).Seconds())
		requests.
			WithLabelValues(version, c.Request.Method, route, status).
			Inc()
	}
}
