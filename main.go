package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/rmitchellscott/aviary-backend/internal/webhook"
)

//go:embed ui/out
//go:embed ui/out/_next
var embeddedUI embed.FS

func main() {
	// Load .env if present
	_ = godotenv.Load()

	// Determine port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	addr := ":" + port

	// 2) Create a Sub FS rooted at our static export
	uiFS, err := fs.Sub(embeddedUI, "ui/out")
	if err != nil {
		log.Fatalf("embed error: %v", err)
	}

	// 3) Gin router for /api
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.POST("/api/webhook", webhook.EnqueueHandler)
	router.GET("/api/status/:id", webhook.StatusHandler)
	router.GET("/api/config", func(c *gin.Context) {
		c.JSON(200, gin.H{"apiUrl": "/api/"})
	})

	// File server for all embedded files
	router.NoRoute(func(c *gin.Context) {
		// strip leading slash
		p := strings.TrimPrefix(c.Request.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}

		// Check if file exists in embedded FS
		if stat, err := fs.Stat(uiFS, p); err != nil || stat.IsDir() {
			p = "index.html"
		}

		http.ServeFileFS(c.Writer, c.Request, uiFS, p)
	})

	log.Printf("Listening on %s…", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}
