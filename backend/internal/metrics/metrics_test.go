package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IBM/pgxpoolprometheus"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBusiness_SeriesInitializedWithZero(t *testing.T) {
	t.Parallel()

	reg := prometheus.NewRegistry()
	NewBusiness(reg)

	expected := `
# HELP urlshort_links_created_total Number of short links created.
# TYPE urlshort_links_created_total counter
urlshort_links_created_total{code_source="custom"} 0
urlshort_links_created_total{code_source="generated"} 0
# HELP urlshort_redirects_total Number of short link resolutions by result.
# TYPE urlshort_redirects_total counter
urlshort_redirects_total{result="found"} 0
urlshort_redirects_total{result="not_found"} 0
`
	require.NoError(t, testutil.GatherAndCompare(reg, strings.NewReader(expected),
		"urlshort_links_created_total", "urlshort_redirects_total"))
}

func TestBusiness_Counters(t *testing.T) {
	t.Parallel()

	b := NewBusiness(prometheus.NewRegistry())

	b.LinkCreated(true)
	b.LinkCreated(false)
	b.LinkCreated(false)
	b.CodeConflict(true)
	b.LinkUpdated()
	b.LinkDeleted()
	b.LinkDeleted()
	b.Redirect(true)
	b.Redirect(true)
	b.Redirect(true)
	b.Redirect(false)

	assert.InDelta(t, 1, testutil.ToFloat64(b.linksCreated.WithLabelValues(codeSourceCustom)), 0)
	assert.InDelta(t, 2, testutil.ToFloat64(b.linksCreated.WithLabelValues(codeSourceGenerated)), 0)
	assert.InDelta(t, 1, testutil.ToFloat64(b.codeConflicts.WithLabelValues(codeSourceCustom)), 0)
	assert.InDelta(t, 0, testutil.ToFloat64(b.codeConflicts.WithLabelValues(codeSourceGenerated)), 0)
	assert.InDelta(t, 1, testutil.ToFloat64(b.linksUpdated), 0)
	assert.InDelta(t, 2, testutil.ToFloat64(b.linksDeleted), 0)
	assert.InDelta(t, 3, testutil.ToFloat64(b.redirects.WithLabelValues(redirectFound)), 0)
	assert.InDelta(t, 1, testutil.ToFloat64(b.redirects.WithLabelValues(redirectNotFound)), 0)
}

func TestHandler_ExposesRuntimeExtraAndBusinessMetrics(t *testing.T) {
	t.Parallel()

	pool, err := pgxpool.New(t.Context(), "postgres://user:password@localhost:1/db")
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	reg := NewRegistry(pgxpoolprometheus.NewCollector(pool, map[string]string{"db_name": "shortener"}))
	NewBusiness(reg).LinkCreated(false)

	server := httptest.NewServer(Handler(reg))
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/metrics")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, resp.Body.Close()) })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	for _, name := range []string{
		"go_goroutines",
		"process_resident_memory_bytes",
		`pgxpool_max_conns{db_name="shortener"}`,
		`urlshort_links_created_total{code_source="generated"} 1`,
		"promhttp_metric_handler_requests_total",
	} {
		assert.Contains(t, string(body), name)
	}
}

func TestHandler_OnlyMetricsPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(Handler(prometheus.NewRegistry()))
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
