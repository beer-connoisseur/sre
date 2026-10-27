package middleware

import (
	"errors"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"urlshort/generated/api"
)

var errChaos = errors.New("chaos: injected failure")

func Chaos(errorRate float64, latency time.Duration, skipPaths ...string) gin.HandlerFunc {
	skip := make(map[string]struct{}, len(skipPaths))
	for _, path := range skipPaths {
		skip[path] = struct{}{}
	}

	return func(c *gin.Context) {
		if _, ok := skip[c.Request.URL.Path]; ok {
			c.Next()
			return
		}

		if latency > 0 {
			timer := time.NewTimer(latency)
			select {
			case <-timer.C:
			case <-c.Request.Context().Done():
				timer.Stop()
			}
		}

		if errorRate > 0 && rand.Float64() < errorRate {
			c.Error(errChaos)
			c.AbortWithStatusJSON(http.StatusInternalServerError, api.Error{
				Error: api.ErrorBody{Code: api.ErrorCodeInternalError, Message: "internal server error"},
			})
			return
		}

		c.Next()
	}
}
