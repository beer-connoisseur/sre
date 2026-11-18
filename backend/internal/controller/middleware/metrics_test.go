package middleware

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestMetrics_RequestsTotalCarriesVersion(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	reg := prometheus.NewRegistry()

	router := gin.New()
	router.Use(Metrics(reg, "9.9.9"))
	router.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })

	for range 3 {
		require.Equal(t, http.StatusOK, serve(router, "/healthz").Code)
	}

	expected := `
# HELP http_requests_total Total number of HTTP requests by version, route template and status.
# TYPE http_requests_total counter
http_requests_total{method="GET",route="/healthz",status="200",version="9.9.9"} 3
`
	require.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(expected), "http_requests_total"))
}
