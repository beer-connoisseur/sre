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
)

//go:generate go tool mockgen -source=link.go -destination=mocks/mocks.go -package=mocks

type Service struct {
	repo         linkRepository
	generateCode func() (string, error)
}

func NewService(repo linkRepository) *Service {
	return &Service{
		repo:         repo,
		generateCode: generator.GenerateCode,
	}
}

func (s *Service) Create(ctx context.Context, originalURL, code string) (entity.Link, error) {
	if code != "" {
		return s.repo.Create(ctx, entity.Link{Code: code, OriginalURL: originalURL})
	}

	for range maxCodeAttempts {
		generated, err := s.generateCode()
		if err != nil {
			return entity.Link{}, err
		}

		link, err := s.repo.Create(ctx, entity.Link{Code: generated, OriginalURL: originalURL})
		if errors.Is(err, entity.ErrCodeAlreadyExists) {
			continue
		}

		return link, err
	}

	return entity.Link{}, errors.New("failed to generate unique code")
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
	return s.repo.Update(ctx, id, upd)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	return s.repo.Resolve(ctx, code)
}
