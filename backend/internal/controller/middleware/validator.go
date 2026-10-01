package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	ginmiddleware "github.com/oapi-codegen/gin-middleware"

	"urlshort/generated/api"
)

func OapiValidator() (gin.HandlerFunc, error) {
	spec, err := api.GetSpec()
	if err != nil {
		return nil, err
	}
	spec.Servers = nil

	return ginmiddleware.OapiRequestValidatorWithOptions(spec, &ginmiddleware.Options{
		ErrorHandler:          validationErrorHandler,
		SilenceServersWarning: true,
	}), nil
}

func validationErrorHandler(c *gin.Context, message string, statusCode int) {
	code := api.ErrorCodeValidationError
	if statusCode == http.StatusNotFound {
		code = api.ErrorCodeNotFound
	}

	c.AbortWithStatusJSON(statusCode, api.Error{
		Error: api.ErrorBody{Code: code, Message: message},
	})
}
