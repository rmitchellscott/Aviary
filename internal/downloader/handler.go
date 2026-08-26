package downloader

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rmitchellscott/aviary/internal/security"
)

// SniffHandler responds with the MIME type of the ?url parameter.
func SniffHandler(c *gin.Context) {
	urlStr := c.Query("url")
	if urlStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "backend.errors.missing_url"})
		return
	}

	mt, err := SniffMime(urlStr)
	if err != nil {
		if security.IsBlockedAddress(err) {
			c.JSON(http.StatusForbidden, gin.H{"error": "backend.status.blocked_address"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "backend.status.internal_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"mime": mt})
}
