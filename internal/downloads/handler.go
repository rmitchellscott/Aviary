package downloads

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rmitchellscott/aviary/internal/storage"
)

func DownloadHandler(c *gin.Context) {
	token := c.Param("token")
	entry, err := Get(token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Download not found or expired"})
		return
	}

	ctx := c.Request.Context()
	if err := storage.StreamToResponse(ctx, c, entry.StorageKey, entry.Filename, ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file"})
	}
}
