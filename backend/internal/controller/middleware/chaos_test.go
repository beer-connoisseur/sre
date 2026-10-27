package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newChaosRouter(errorRate float64, latency time.Duration) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Chaos(errorRate, latency, "/healthz"))
	router.GET("/r/:code", func(c *gin.Context) { c.Status(http.StatusFound) })
	router.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })

	return router
}

func serve(router http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))

	return rec
}

func TestChaos_DisabledPassesThrough(t *testing.T) {
	t.Parallel()

	router := newChaosRouter(0, 0)
	for range 100 {
		require.Equal(t, http.StatusFound, serve(router, "/r/abc").Code)
	}
}

func TestChaos_FullErrorRateAlwaysFails(t *testing.T) {
	t.Parallel()

	router := newChaosRouter(1, 0)
	for range 20 {
		rec := serve(router, "/r/abc")
		require.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.JSONEq(t, `{"error":{"code":"internal_error","message":"internal server error"}}`, rec.Body.String())
	}
}

func TestChaos_PartialErrorRate(t *testing.T) {
	t.Parallel()

	router := newChaosRouter(0.5, 0)
	failures := 0
	const total = 1000
	for range total {
		if serve(router, "/r/abc").Code == http.StatusInternalServerError {
			failures++
		}
	}

	assert.InDelta(t, total/2, failures, total/10)
}

func TestChaos_Latency(t *testing.T) {
	t.Parallel()

	router := newChaosRouter(0, 50*time.Millisecond)

	start := time.Now()
	require.Equal(t, http.StatusFound, serve(router, "/r/abc").Code)
	assert.GreaterOrEqual(t, time.Since(start), 50*time.Millisecond)
}

func TestChaos_SkipsProbes(t *testing.T) {
	t.Parallel()

	router := newChaosRouter(1, time.Second)

	start := time.Now()
	require.Equal(t, http.StatusOK, serve(router, "/healthz").Code)
	assert.Less(t, time.Since(start), 500*time.Millisecond)
}
