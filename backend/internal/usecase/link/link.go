package link

import (
	"context"
	"errors"

	"urlshort/internal/entity"
	"urlshort/internal/usecase/generator"
)

const maxCodeAttempts = 5

type (
	linkRepository interface {
		Create(ctx context.Context, link entity.Link) (entity.Link, error)
		GetByID(ctx context.Context, id string) (entity.Link, error)
		List(ctx context.Context, limit, offset int) ([]entity.Link, int, error)
		Update(ctx context.Context, id string, upd entity.LinkUpdate) (entity.Link, error)
		Delete(ctx context.Context, id string) error
		Resolve(ctx context.Context, code string) (string, error)
	}

	linkMetrics interface {
		LinkCreated(customCode bool)
		CodeConflict(customCode bool)
		LinkUpdated()
		LinkDeleted()
		Redirect(found bool)
	}
)

//go:generate go tool mockgen -source=link.go -destination=mocks/mocks.go -package=mocks

type Service struct {
	repo         linkRepository
	metrics      linkMetrics
	generateCode func() (string, error)
}

func NewService(repo linkRepository, metrics linkMetrics) *Service {
	return &Service{
		repo:         repo,
		metrics:      metrics,
		generateCode: generator.GenerateCode,
	}
}

func (s *Service) Create(ctx context.Context, originalURL, code string) (entity.Link, error) {
	if code != "" {
		return s.create(ctx, entity.Link{Code: code, OriginalURL: originalURL}, true)
	}

	for range maxCodeAttempts {
		generated, err := s.generateCode()
		if err != nil {
			return entity.Link{}, err
		}

		link, err := s.create(ctx, entity.Link{Code: generated, OriginalURL: originalURL}, false)
		if errors.Is(err, entity.ErrCodeAlreadyExists) {
			continue
		}

		return link, err
	}

	return entity.Link{}, errors.New("failed to generate unique code")
}

func (s *Service) create(ctx context.Context, link entity.Link, customCode bool) (entity.Link, error) {
	created, err := s.repo.Create(ctx, link)
	switch {
	case err == nil:
		s.metrics.LinkCreated(customCode)
	case errors.Is(err, entity.ErrCodeAlreadyExists):
		s.metrics.CodeConflict(customCode)
	}

	return created, err
}

func (s *Service) Get(ctx context.Context, id string) (entity.Link, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context, limit, offset int) (entity.LinkPage, error) {
	items, total, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return entity.LinkPage{}, err
	}

	return entity.LinkPage{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (s *Service) Update(ctx context.Context, id string, upd entity.LinkUpdate) (entity.Link, error) {
	link, err := s.repo.Update(ctx, id, upd)
	switch {
	case err == nil:
		s.metrics.LinkUpdated()
	case errors.Is(err, entity.ErrCodeAlreadyExists):
		s.metrics.CodeConflict(true)
	}

	return link, err
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	s.metrics.LinkDeleted()

	return nil
}

func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	originalURL, err := s.repo.Resolve(ctx, code)
	switch {
	case err == nil:
		s.metrics.Redirect(true)
	case errors.Is(err, entity.ErrLinkNotFound):
		s.metrics.Redirect(false)
	}

	return originalURL, err
}
