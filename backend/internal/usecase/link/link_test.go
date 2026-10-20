package link

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"urlshort/internal/entity"
	"urlshort/internal/usecase/link/mocks"
)

const originalURL = "https://example.com"

type testEnv struct {
	svc     *Service
	repo    *mocks.MocklinkRepository
	metrics *mocks.MocklinkMetrics
}

func newTestEnv(t *testing.T) testEnv {
	t.Helper()

	ctrl := gomock.NewController(t)
	repo := mocks.NewMocklinkRepository(ctrl)
	metrics := mocks.NewMocklinkMetrics(ctrl)

	return testEnv{svc: NewService(repo, metrics), repo: repo, metrics: metrics}
}

func stubCodes(svc *Service, codes ...string) {
	svc.generateCode = func() (string, error) {
		code := codes[0]
		codes = codes[1:]
		return code, nil
	}
}

func TestCreate_GeneratesCode(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	stubCodes(env.svc, "abcd1234")

	want := entity.Link{ID: "1", Code: "abcd1234", OriginalURL: originalURL}
	env.repo.EXPECT().
		Create(gomock.Any(), entity.Link{Code: "abcd1234", OriginalURL: originalURL}).
		Return(want, nil)
	env.metrics.EXPECT().LinkCreated(false)

	got, err := env.svc.Create(context.Background(), originalURL, "")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestCreate_GeneratedCodeHasExpectedLength(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)

	env.repo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, link entity.Link) (entity.Link, error) {
			return link, nil
		})
	env.metrics.EXPECT().LinkCreated(false)

	got, err := env.svc.Create(context.Background(), originalURL, "")
	require.NoError(t, err)
	assert.Len(t, got.Code, 8)
}

func TestCreate_RetriesOnCodeCollision(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	stubCodes(env.svc, "taken1", "taken2", "free1")

	gomock.InOrder(
		env.repo.EXPECT().
			Create(gomock.Any(), entity.Link{Code: "taken1", OriginalURL: originalURL}).
			Return(entity.Link{}, entity.ErrCodeAlreadyExists),
		env.repo.EXPECT().
			Create(gomock.Any(), entity.Link{Code: "taken2", OriginalURL: originalURL}).
			Return(entity.Link{}, entity.ErrCodeAlreadyExists),
		env.repo.EXPECT().
			Create(gomock.Any(), entity.Link{Code: "free1", OriginalURL: originalURL}).
			Return(entity.Link{Code: "free1", OriginalURL: originalURL}, nil),
	)
	env.metrics.EXPECT().CodeConflict(false).Times(2)
	env.metrics.EXPECT().LinkCreated(false)

	got, err := env.svc.Create(context.Background(), originalURL, "")
	require.NoError(t, err)
	assert.Equal(t, "free1", got.Code)
}

func TestCreate_GivesUpAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.svc.generateCode = func() (string, error) { return "taken", nil }

	env.repo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(entity.Link{}, entity.ErrCodeAlreadyExists).
		Times(maxCodeAttempts)
	env.metrics.EXPECT().CodeConflict(false).Times(maxCodeAttempts)

	_, err := env.svc.Create(context.Background(), originalURL, "")
	require.Error(t, err)
}

func TestCreate_GeneratorError(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.repo.EXPECT().Create(gomock.Any(), gomock.Any()).Times(0)
	genErr := errors.New("no entropy")
	env.svc.generateCode = func() (string, error) { return "", genErr }

	_, err := env.svc.Create(context.Background(), originalURL, "")
	require.ErrorIs(t, err, genErr)
}

func TestCreate_CustomCode(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.svc.generateCode = func() (string, error) {
		t.Error("generator must not be called for a custom code")
		return "", nil
	}

	want := entity.Link{ID: "1", Code: "my-link", OriginalURL: originalURL}
	env.repo.EXPECT().
		Create(gomock.Any(), entity.Link{Code: "my-link", OriginalURL: originalURL}).
		Return(want, nil)
	env.metrics.EXPECT().LinkCreated(true)

	got, err := env.svc.Create(context.Background(), originalURL, "my-link")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestCreate_CustomCodeConflict(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.svc.generateCode = func() (string, error) {
		t.Error("generator must not be called for a custom code")
		return "", nil
	}

	env.repo.EXPECT().
		Create(gomock.Any(), entity.Link{Code: "my-link", OriginalURL: originalURL}).
		Return(entity.Link{}, entity.ErrCodeAlreadyExists)
	env.metrics.EXPECT().CodeConflict(true)

	_, err := env.svc.Create(context.Background(), originalURL, "my-link")
	require.ErrorIs(t, err, entity.ErrCodeAlreadyExists)
}

func TestList(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	items := []entity.Link{{ID: "1"}}
	env.repo.EXPECT().List(gomock.Any(), 50, 10).Return(items, 7, nil)

	page, err := env.svc.List(context.Background(), 50, 10)
	require.NoError(t, err)
	assert.Equal(t, entity.LinkPage{Items: items, Total: 7, Limit: 50, Offset: 10}, page)
}

func TestList_RepositoryError(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	dbErr := errors.New("db is down")
	env.repo.EXPECT().List(gomock.Any(), 20, 0).Return(nil, 0, dbErr)

	_, err := env.svc.List(context.Background(), 20, 0)
	require.ErrorIs(t, err, dbErr)
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	code := "new-code"
	upd := entity.LinkUpdate{Code: &code}
	env.repo.EXPECT().Update(gomock.Any(), "1", upd).Return(entity.Link{ID: "1", Code: code}, nil)
	env.metrics.EXPECT().LinkUpdated()

	got, err := env.svc.Update(context.Background(), "1", upd)
	require.NoError(t, err)
	assert.Equal(t, code, got.Code)
}

func TestResolve(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.repo.EXPECT().Resolve(gomock.Any(), "abc").Return(originalURL, nil)
	env.repo.EXPECT().Resolve(gomock.Any(), "missing").Return("", entity.ErrLinkNotFound)
	env.metrics.EXPECT().Redirect(true)
	env.metrics.EXPECT().Redirect(false)

	got, err := env.svc.Resolve(context.Background(), "abc")
	require.NoError(t, err)
	assert.Equal(t, originalURL, got)

	_, err = env.svc.Resolve(context.Background(), "missing")
	require.ErrorIs(t, err, entity.ErrLinkNotFound)
}

func TestCreate_RepositoryErrorIsNotCounted(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	dbErr := errors.New("db is down")
	env.repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(entity.Link{}, dbErr)

	_, err := env.svc.Create(context.Background(), originalURL, "my-link")
	require.ErrorIs(t, err, dbErr)
}

func TestUpdate_CodeConflict(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	code := "taken"
	upd := entity.LinkUpdate{Code: &code}
	env.repo.EXPECT().Update(gomock.Any(), "1", upd).Return(entity.Link{}, entity.ErrCodeAlreadyExists)
	env.metrics.EXPECT().CodeConflict(true)

	_, err := env.svc.Update(context.Background(), "1", upd)
	require.ErrorIs(t, err, entity.ErrCodeAlreadyExists)
}

func TestDelete(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.repo.EXPECT().Delete(gomock.Any(), "1").Return(nil)
	env.metrics.EXPECT().LinkDeleted()

	require.NoError(t, env.svc.Delete(context.Background(), "1"))
}

func TestDelete_NotFoundIsNotCounted(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	env.repo.EXPECT().Delete(gomock.Any(), "missing").Return(entity.ErrLinkNotFound)

	require.ErrorIs(t, env.svc.Delete(context.Background(), "missing"), entity.ErrLinkNotFound)
}

func TestResolve_RepositoryErrorIsNotCounted(t *testing.T) {
	t.Parallel()

	env := newTestEnv(t)
	dbErr := errors.New("db is down")
	env.repo.EXPECT().Resolve(gomock.Any(), "abc").Return("", dbErr)

	_, err := env.svc.Resolve(context.Background(), "abc")
	require.ErrorIs(t, err, dbErr)
}
