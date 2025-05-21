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

// 1) Embed index.html plus everything under _next/ recursively
//    (change static/ if your export has other top-level folders)

//go:embed assets/ui/out/index.html
//go:embed assets/ui/out/_next/**
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
	uiFS, err := fs.Sub(embeddedUI, "assets/ui/out")
	if err != nil {
		log.Fatalf("embed error: %v", err)
	}

	// 3) Gin router for /api
	apiRouter := gin.New()
	apiRouter.Use(gin.Logger(), gin.Recovery())
	// apiRouter.POST("/api/webhook", webhook.Handler)
	// apiRouter.GET("/api/config", func(c *gin.Context) {
	// 	c.JSON(200, gin.H{"apiUrl": "/api/"})
	// })
	apiRouter.POST("/api/webhook", webhook.EnqueueHandler)
	apiRouter.GET("/api/status/:id", webhook.StatusHandler)
	apiRouter.GET("/api/config", func(c *gin.Context) {
		c.JSON(200, gin.H{"apiUrl": "/api/"})
	})

	// 4) http.ServeMux for static + SPA fallback
	mux := http.NewServeMux()
	mux.Handle("/api/", apiRouter)

	// File server for all embedded files
	fileServer := http.FileServer(http.FS(uiFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// strip leading slash
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}

		// Check if file exists in embedded FS
		if f, err := uiFS.Open(p); err == nil {
			if info, _ := f.Stat(); !info.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// Fallback to index.html for SPA
		data, err := fs.ReadFile(uiFS, "index.html")
		if err != nil {
			http.Error(w, "index.html not found", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	log.Printf("Listening on %s…", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
