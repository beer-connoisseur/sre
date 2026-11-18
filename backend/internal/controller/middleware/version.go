package middleware

import (
	"github.com/gin-gonic/gin"
)

func Version(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Version", version)
		c.Next()
	}
}
