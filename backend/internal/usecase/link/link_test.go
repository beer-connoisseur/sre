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

func newService(t *testing.T) (*Service, *mocks.MocklinkRepository) {
	t.Helper()

	repo := mocks.NewMocklinkRepository(gomock.NewController(t))

	return NewService(repo), repo
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

	svc, repo := newService(t)
	stubCodes(svc, "abcd1234")

	want := entity.Link{ID: "1", Code: "abcd1234", OriginalURL: originalURL}
	repo.EXPECT().
		Create(gomock.Any(), entity.Link{Code: "abcd1234", OriginalURL: originalURL}).
		Return(want, nil)

	got, err := svc.Create(context.Background(), originalURL, "")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestCreate_GeneratedCodeHasExpectedLength(t *testing.T) {
	t.Parallel()

	svc, repo := newService(t)

	repo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, link entity.Link) (entity.Link, error) {
			return link, nil
		})

	got, err := svc.Create(context.Background(), originalURL, "")
	require.NoError(t, err)
	assert.Len(t, got.Code, 8)
}

func TestCreate_RetriesOnCodeCollision(t *testing.T) {
	t.Parallel()

	svc, repo := newService(t)
	stubCodes(svc, "taken1", "taken2", "free1")

	gomock.InOrder(
		repo.EXPECT().
			Create(gomock.Any(), entity.Link{Code: "taken1", OriginalURL: originalURL}).
			Return(entity.Link{}, entity.ErrCodeAlreadyExists),
		repo.EXPECT().
			Create(gomock.Any(), entity.Link{Code: "taken2", OriginalURL: originalURL}).
			Return(entity.Link{}, entity.ErrCodeAlreadyExists),
		repo.EXPECT().
			Create(gomock.Any(), entity.Link{Code: "free1", OriginalURL: originalURL}).
			Return(entity.Link{Code: "free1", OriginalURL: originalURL}, nil),
	)

	got, err := svc.Create(context.Background(), originalURL, "")
	require.NoError(t, err)
	assert.Equal(t, "free1", got.Code)
}

func TestCreate_GivesUpAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	svc, repo := newService(t)
	svc.generateCode = func() (string, error) { return "taken", nil }

	repo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(entity.Link{}, entity.ErrCodeAlreadyExists).
		Times(maxCodeAttempts)

	_, err := svc.Create(context.Background(), originalURL, "")
	require.Error(t, err)
}

func TestCreate_GeneratorError(t *testing.T) {
	t.Parallel()

	svc, repo := newService(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Times(0)
	genErr := errors.New("no entropy")
	svc.generateCode = func() (string, error) { return "", genErr }

	_, err := svc.Create(context.Background(), originalURL, "")
	require.ErrorIs(t, err, genErr)
}

func TestCreate_CustomCode(t *testing.T) {
	t.Parallel()

	svc, repo := newService(t)
	svc.generateCode = func() (string, error) {
		t.Error("generator must not be called for a custom code")
		return "", nil
	}

	repo.EXPECT().
		Create(gomock.Any(), entity.Link{Code: "my-link", OriginalURL: originalURL}).
		Return(entity.Link{}, entity.ErrCodeAlreadyExists)

	_, err := svc.Create(context.Background(), originalURL, "my-link")
	require.ErrorIs(t, err, entity.ErrCodeAlreadyExists)
}

func TestList(t *testing.T) {
	t.Parallel()

	svc, repo := newService(t)
	items := []entity.Link{{ID: "1"}}
	repo.EXPECT().List(gomock.Any(), 50, 10).Return(items, 7, nil)

	page, err := svc.List(context.Background(), 50, 10)
	require.NoError(t, err)
	assert.Equal(t, entity.LinkPage{Items: items, Total: 7, Limit: 50, Offset: 10}, page)
}

func TestList_RepositoryError(t *testing.T) {
	t.Parallel()

	svc, repo := newService(t)
	dbErr := errors.New("db is down")
	repo.EXPECT().List(gomock.Any(), 20, 0).Return(nil, 0, dbErr)

	_, err := svc.List(context.Background(), 20, 0)
	require.ErrorIs(t, err, dbErr)
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	svc, repo := newService(t)
	code := "new-code"
	upd := entity.LinkUpdate{Code: &code}
	repo.EXPECT().Update(gomock.Any(), "1", upd).Return(entity.Link{ID: "1", Code: code}, nil)

	got, err := svc.Update(context.Background(), "1", upd)
	require.NoError(t, err)
	assert.Equal(t, code, got.Code)
}

func TestResolve(t *testing.T) {
	t.Parallel()

	svc, repo := newService(t)
	repo.EXPECT().Resolve(gomock.Any(), "abc").Return(originalURL, nil)
	repo.EXPECT().Resolve(gomock.Any(), "missing").Return("", entity.ErrLinkNotFound)

	got, err := svc.Resolve(context.Background(), "abc")
	require.NoError(t, err)
	assert.Equal(t, originalURL, got)

	_, err = svc.Resolve(context.Background(), "missing")
	require.ErrorIs(t, err, entity.ErrLinkNotFound)
}
