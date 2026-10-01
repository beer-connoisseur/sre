package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"urlshort/generated/api"
	"urlshort/internal/entity"
)

const pingTimeout = 2 * time.Second

type (
	linkUseCase interface {
		Create(ctx context.Context, originalURL, code string) (entity.Link, error)
		Get(ctx context.Context, id string) (entity.Link, error)
		List(ctx context.Context, limit, offset int) (entity.LinkPage, error)
		Update(ctx context.Context, id string, upd entity.LinkUpdate) (entity.Link, error)
		Delete(ctx context.Context, id string) error
		Resolve(ctx context.Context, code string) (string, error)
	}

	pinger interface {
		Ping(ctx context.Context) error
	}
)

//go:generate go tool mockgen -source=link.go -destination=mocks/mocks.go -package=mocks

var _ api.ServerInterface = (*serviceImplementation)(nil)

type serviceImplementation struct {
	logger      *zap.Logger
	linkUseCase linkUseCase
	db          pinger
}

func New(logger *zap.Logger, linkUseCase linkUseCase, db pinger) *serviceImplementation {
	return &serviceImplementation{
		logger:      logger,
		linkUseCase: linkUseCase,
		db:          db,
	}
}

func (i *serviceImplementation) ListLinks(c *gin.Context, params api.ListLinksParams) {
	page, err := i.linkUseCase.List(c.Request.Context(), valueOrZero(params.Limit), valueOrZero(params.Offset))
	if err != nil {
		i.respondError(c, err)
		return
	}

	items := make([]api.Link, 0, len(page.Items))
	for _, link := range page.Items {
		items = append(items, toAPILink(link))
	}

	c.JSON(http.StatusOK, api.LinkPage{
		Items:  items,
		Total:  page.Total,
		Limit:  page.Limit,
		Offset: page.Offset,
	})
}

func (i *serviceImplementation) CreateLink(c *gin.Context) {
	var req api.CreateLinkJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, newError(api.ErrorCodeBadRequest, err.Error()))
		return
	}

	link, err := i.linkUseCase.Create(c.Request.Context(), req.OriginalUrl, valueOrZero(req.Code))
	if err != nil {
		i.respondError(c, err)
		return
	}

	c.Header("Location", "/api/v1/links/"+link.ID)
	c.JSON(http.StatusCreated, toAPILink(link))
}

func (i *serviceImplementation) GetLink(c *gin.Context, id api.LinkID) {
	link, err := i.linkUseCase.Get(c.Request.Context(), id)
	if err != nil {
		i.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, toAPILink(link))
}

func (i *serviceImplementation) UpdateLink(c *gin.Context, id api.LinkID) {
	var req api.UpdateLinkJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, newError(api.ErrorCodeBadRequest, err.Error()))
		return
	}

	link, err := i.linkUseCase.Update(c.Request.Context(), id, entity.LinkUpdate{
		OriginalURL: req.OriginalUrl,
		Code:        req.Code,
	})
	if err != nil {
		i.respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, toAPILink(link))
}

func (i *serviceImplementation) DeleteLink(c *gin.Context, id api.LinkID) {
	if err := i.linkUseCase.Delete(c.Request.Context(), id); err != nil {
		i.respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (i *serviceImplementation) Redirect(c *gin.Context, code api.Code) {
	originalURL, err := i.linkUseCase.Resolve(c.Request.Context(), code)
	if err != nil {
		i.respondError(c, err)
		return
	}

	c.Redirect(http.StatusFound, originalURL)
}

func (i *serviceImplementation) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, api.Status{Status: api.StatusStatusOk})
}

func (i *serviceImplementation) Readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), pingTimeout)
	defer cancel()

	if err := i.db.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, api.Status{Status: api.StatusStatusUnavailable})
		return
	}

	c.JSON(http.StatusOK, api.Status{Status: api.StatusStatusOk})
}
