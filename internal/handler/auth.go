package handler

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequireAdminToken guards the admin surface (job triggers and the category/user
// CRUD) with a shared bearer token.
//
// These routes can delete data, list every user, and kick off jobs that burn the
// GitHub API quota, so they must never be reachable anonymously once the API is
// on the public internet. The public frontend only reads GET /trending*, which
// stays open.
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
