package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"urlshort/generated/api"
	"urlshort/internal/entity"
)

func convertErrors(err error) (int, api.Error) {
	switch {
	case errors.Is(err, entity.ErrLinkNotFound):
		return http.StatusNotFound, newError(api.ErrorCodeNotFound, err.Error())

	case errors.Is(err, entity.ErrCodeAlreadyExists):
		return http.StatusConflict, newError(api.ErrorCodeCodeTaken, err.Error())

	default:
		return http.StatusInternalServerError, newError(api.ErrorCodeInternalError, "internal server error")
	}
}

func (i *serviceImplementation) respondError(c *gin.Context, err error) {
	status, body := convertErrors(err)
	if status == http.StatusInternalServerError {
		i.logger.Error("request failed", zap.String("path", c.Request.URL.Path), zap.Error(err))
	}

	c.JSON(status, body)
}

func newError(code api.ErrorCode, message string) api.Error {
	return api.Error{Error: api.ErrorBody{Code: code, Message: message}}
}

func toAPILink(link entity.Link) api.Link {
	return api.Link{
		Id:          link.ID,
		Code:        link.Code,
		OriginalUrl: link.OriginalURL,
		Clicks:      link.Clicks,
		CreatedAt:   link.CreatedAt,
		UpdatedAt:   link.UpdatedAt,
	}
}

func valueOrZero[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}

	return *p
}
