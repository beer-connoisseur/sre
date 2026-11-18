package middleware

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersion_HeaderOnEveryResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Version("1.2.3"))
	router.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/boom", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })

	for _, target := range []string{"/healthz", "/boom"} {
		rec := serve(router, target)
		require.Equal(t, target == "/boom", rec.Code == http.StatusInternalServerError)
		assert.Equal(t, "1.2.3", rec.Header().Get("X-Version"), "route %s", target)
	}
}
