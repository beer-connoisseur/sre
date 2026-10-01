package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"urlshort/generated/api"
	"urlshort/internal/controller/middleware"
	"urlshort/internal/controller/mocks"
	"urlshort/internal/entity"
)

const (
	linkID      = "0b6c1d8e-7a51-4d2c-9f3e-2a4b5c6d7e8f"
	originalURL = "https://example.com"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

type testEnv struct {
	router  http.Handler
	useCase *mocks.MocklinkUseCase
	db      *mocks.Mockpinger
}

func newTestEnv(t *testing.T) testEnv {
	t.Helper()

	ctrl := gomock.NewController(t)
	useCase := mocks.NewMocklinkUseCase(ctrl)
	db := mocks.NewMockpinger(ctrl)

	validator, err := middleware.OapiValidator()
	require.NoError(t, err)

	router := gin.New()
	router.Use(validator)
	api.RegisterHandlers(router, New(zap.NewNop(), useCase, db))

	return testEnv{router: router, useCase: useCase, db: db}
}

func (e testEnv) do(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)

	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) api.ErrorCode {
	t.Helper()

	var body api.Error
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	return body.Error.Code
}

func TestListLinks(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.useCase.EXPECT().
		List(gomock.Any(), 5, 10).
		Return(entity.LinkPage{Items: []entity.Link{{ID: linkID, Code: "abc"}}, Total: 11, Limit: 5, Offset: 10}, nil)

	rec := env.do(t, http.MethodGet, "/api/v1/links?limit=5&offset=10", "")
	require.Equal(t, http.StatusOK, rec.Code)

	var body api.LinkPage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, 11, body.Total)
	require.Len(t, body.Items, 1)
	assert.Equal(t, "abc", body.Items[0].Code)
}

func TestListLinks_DefaultParams(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.useCase.EXPECT().List(gomock.Any(), 20, 0).Return(entity.LinkPage{Limit: 20}, nil)

	rec := env.do(t, http.MethodGet, "/api/v1/links", "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCreateLink(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.useCase.EXPECT().
		Create(gomock.Any(), originalURL, "").
		Return(entity.Link{ID: linkID, Code: "abc", OriginalURL: originalURL}, nil)

	rec := env.do(t, http.MethodPost, "/api/v1/links", `{"originalUrl":"`+originalURL+`"}`)
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "/api/v1/links/"+linkID, rec.Header().Get("Location"))

	var body api.Link
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "abc", body.Code)
}

func TestCreateLink_CustomCode(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.useCase.EXPECT().
		Create(gomock.Any(), originalURL, "my-link").
		Return(entity.Link{ID: linkID, Code: "my-link", OriginalURL: originalURL}, nil)

	rec := env.do(t, http.MethodPost, "/api/v1/links", `{"originalUrl":"`+originalURL+`","code":"my-link"}`)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestGetLink(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.useCase.EXPECT().Get(gomock.Any(), linkID).Return(entity.Link{ID: linkID, Code: "abc"}, nil)

	rec := env.do(t, http.MethodGet, "/api/v1/links/"+linkID, "")
	require.Equal(t, http.StatusOK, rec.Code)

	var body api.Link
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, linkID, body.Id)
}

func TestUpdateLink(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	code := "new-code"
	env.useCase.EXPECT().
		Update(gomock.Any(), linkID, entity.LinkUpdate{Code: &code}).
		Return(entity.Link{ID: linkID, Code: code}, nil)

	rec := env.do(t, http.MethodPatch, "/api/v1/links/"+linkID, `{"code":"new-code"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	var body api.Link
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, code, body.Code)
}

func TestDeleteLink(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.useCase.EXPECT().Delete(gomock.Any(), linkID).Return(nil)

	rec := env.do(t, http.MethodDelete, "/api/v1/links/"+linkID, "")
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRequestValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "unknown field", method: http.MethodPost, target: "/api/v1/links", body: `{"url":"https://example.com"}`},
		{name: "missing url", method: http.MethodPost, target: "/api/v1/links", body: `{"code":"abc"}`},
		{name: "not http url", method: http.MethodPost, target: "/api/v1/links", body: `{"originalUrl":"javascript:alert(1)"}`},
		{name: "bad code", method: http.MethodPost, target: "/api/v1/links", body: `{"originalUrl":"https://example.com","code":"a/b"}`},
		{name: "empty update", method: http.MethodPatch, target: "/api/v1/links/" + linkID, body: `{}`},
		{name: "id is not uuid", method: http.MethodGet, target: "/api/v1/links/not-a-uuid"},
		{name: "limit too big", method: http.MethodGet, target: "/api/v1/links?limit=1000"},
		{name: "limit not a number", method: http.MethodGet, target: "/api/v1/links?limit=ten"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := newTestEnv(t)

			rec := env.do(t, tt.method, tt.target, tt.body)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.Equal(t, api.ErrorCodeValidationError, errorCode(t, rec))
		})
	}
}

func TestErrorMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err        error
		wantStatus int
		wantCode   api.ErrorCode
	}{
		{err: entity.ErrLinkNotFound, wantStatus: http.StatusNotFound, wantCode: api.ErrorCodeNotFound},
		{err: entity.ErrCodeAlreadyExists, wantStatus: http.StatusConflict, wantCode: api.ErrorCodeCodeTaken},
		{err: errors.New("db is down"), wantStatus: http.StatusInternalServerError, wantCode: api.ErrorCodeInternalError},
	}

	for _, tt := range tests {
		t.Run(string(tt.wantCode), func(t *testing.T) {
			t.Parallel()

			env := newTestEnv(t)
			env.useCase.EXPECT().Update(gomock.Any(), linkID, gomock.Any()).Return(entity.Link{}, tt.err)

			rec := env.do(t, http.MethodPatch, "/api/v1/links/"+linkID, `{"code":"new-code"}`)
			require.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantCode, errorCode(t, rec))
			assert.NotContains(t, rec.Body.String(), "db is down")
		})
	}
}

func TestUnknownRoute(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	rec := env.do(t, http.MethodGet, "/api/v1/unknown", "")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, api.ErrorCodeNotFound, errorCode(t, rec))
}

func TestRedirect(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.useCase.EXPECT().Resolve(gomock.Any(), "abc").Return(originalURL, nil)
	env.useCase.EXPECT().Resolve(gomock.Any(), "nope").Return("", entity.ErrLinkNotFound)

	rec := env.do(t, http.MethodGet, "/r/abc", "")
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, originalURL, rec.Header().Get("Location"))

	rec = env.do(t, http.MethodGet, "/r/nope", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHealthz(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	rec := env.do(t, http.MethodGet, "/healthz", "")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestReadyz(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pingErr    error
		wantStatus int
	}{
		{name: "database is up", pingErr: nil, wantStatus: http.StatusOK},
		{name: "database is down", pingErr: errors.New("down"), wantStatus: http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := newTestEnv(t)
			env.db.EXPECT().Ping(gomock.Any()).Return(tt.pingErr)

			rec := env.do(t, http.MethodGet, "/readyz", "")
			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}
