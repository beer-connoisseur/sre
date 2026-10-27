package prober

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"urlshort/generated/api"
)

type fakeAPI struct {
	mu             sync.Mutex
	links          map[string]string
	createStatus   int
	redirectStatus int
	deleted        []string
	userAgents     []string
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{links: map[string]string{}, createStatus: http.StatusCreated, redirectStatus: http.StatusFound}
}

func (f *fakeAPI) handler(t *testing.T) http.Handler {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/links", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.userAgents = append(f.userAgents, r.UserAgent())

		if f.createStatus != http.StatusCreated {
			w.WriteHeader(f.createStatus)
			return
		}

		var req api.CreateLinkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		f.links["abc"] = req.OriginalUrl
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(api.Link{Id: "id-1", Code: "abc", OriginalUrl: req.OriginalUrl}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	})
	mux.HandleFunc("GET /r/{code}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		if f.redirectStatus != http.StatusFound {
			w.WriteHeader(f.redirectStatus)
			return
		}
		http.Redirect(w, r, f.links[r.PathValue("code")], http.StatusFound)
	})
	mux.HandleFunc("DELETE /api/v1/links/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		f.deleted = append(f.deleted, r.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	})

	return mux
}

func newTestProber(t *testing.T, fake *fakeAPI) *Prober {
	t.Helper()

	server := httptest.NewServer(fake.handler(t))
	t.Cleanup(server.Close)

	p, err := New(server.URL, "urlshort-prober", time.Second, zaptest.NewLogger(t), prometheus.NewRegistry())
	require.NoError(t, err)

	return p
}

func TestRunOnce_Success(t *testing.T) {
	t.Parallel()

	fake := newFakeAPI()
	p := newTestProber(t, fake)

	p.RunOnce(t.Context())

	assert.InDelta(t, 1, testutil.ToFloat64(p.runs.WithLabelValues(resultSuccess)), 0)
	assert.InDelta(t, 0, testutil.ToFloat64(p.runs.WithLabelValues(resultFailure)), 0)
	assert.InDelta(t, 1, testutil.ToFloat64(p.lastSuccess), 0)
	assert.Equal(t, []string{"id-1"}, fake.deleted)
	assert.Equal(t, []string{"urlshort-prober"}, fake.userAgents)
	assert.Equal(t, 3, testutil.CollectAndCount(p.stepDuration))
}

func TestRunOnce_RedirectFailureStillDeletesLink(t *testing.T) {
	t.Parallel()

	fake := newFakeAPI()
	fake.redirectStatus = http.StatusInternalServerError
	p := newTestProber(t, fake)

	p.RunOnce(t.Context())

	assert.InDelta(t, 1, testutil.ToFloat64(p.runs.WithLabelValues(resultFailure)), 0)
	assert.InDelta(t, 1, testutil.ToFloat64(p.stepFailures.WithLabelValues(stepRedirect)), 0)
	assert.InDelta(t, 0, testutil.ToFloat64(p.stepFailures.WithLabelValues(stepDelete)), 0)
	assert.InDelta(t, 0, testutil.ToFloat64(p.lastSuccess), 0)
	assert.Equal(t, []string{"id-1"}, fake.deleted)
}

func TestRunOnce_CreateFailureSkipsOtherSteps(t *testing.T) {
	t.Parallel()

	fake := newFakeAPI()
	fake.createStatus = http.StatusInternalServerError
	p := newTestProber(t, fake)

	p.RunOnce(t.Context())

	assert.InDelta(t, 1, testutil.ToFloat64(p.runs.WithLabelValues(resultFailure)), 0)
	assert.InDelta(t, 1, testutil.ToFloat64(p.stepFailures.WithLabelValues(stepCreate)), 0)
	assert.Empty(t, fake.deleted)
}

func TestRunOnce_UnavailableTarget(t *testing.T) {
	t.Parallel()

	p, err := New("http://127.0.0.1:1", "urlshort-prober", time.Second, zaptest.NewLogger(t), prometheus.NewRegistry())
	require.NoError(t, err)

	p.RunOnce(t.Context())

	assert.InDelta(t, 1, testutil.ToFloat64(p.runs.WithLabelValues(resultFailure)), 0)
	assert.InDelta(t, 1, testutil.ToFloat64(p.stepFailures.WithLabelValues(stepCreate)), 0)
}
