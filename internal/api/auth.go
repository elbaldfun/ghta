package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequireAdminToken guards the admin surface (/internal/*) with a shared bearer
// token. It is read-only today, but it reports the worker's internals and must
// not be reachable anonymously on the public internet.
//
// An empty token makes every guarded request fail with 503 rather than passing:
// forgetting to set ADMIN_API_TOKEN takes the admin surface offline instead of
// silently opening it to the world.
func RequireAdminToken(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "admin API disabled: ADMIN_API_TOKEN is not configured",
			})
			return
		}

		presented, ok := bearerToken(c.GetHeader("Authorization"))
		// Constant-time compare so a wrong token can't be recovered by timing.
		if !ok || subtle.ConstantTimeCompare([]byte(presented), []byte(token)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	return strings.TrimSpace(header[len(prefix):]), true
}
