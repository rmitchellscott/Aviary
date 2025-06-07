package main

import (
	// standard library
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	// third-party
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/term"

	// internal
	"github.com/rmitchellscott/aviary/internal/auth"
	"github.com/rmitchellscott/aviary/internal/webhook"
)

//go:generate npm --prefix ui install
//go:generate npm --prefix ui run build
//go:embed ui/out
//go:embed ui/out/_next
var embeddedUI embed.FS

// authRequired checks if any form of authentication is configured
func authRequired() bool {
	envUsername := os.Getenv("AUTH_USERNAME")
	envPassword := os.Getenv("AUTH_PASSWORD")
	envApiKey := os.Getenv("API_KEY")
	return (envUsername != "" && envPassword != "") || envApiKey != ""
}

func main() {
	// Load .env if present
	_ = godotenv.Load()

	// Interactive pair flow
	if len(os.Args) > 1 && os.Args[1] == "pair" {
		if err := runPair(os.Stdout, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "pair failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Determine port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	addr := ":" + port

	// Check for rmapi.conf
	home, err := os.UserHomeDir()
	if err == nil {
		cfgPath := filepath.Join(home, ".config", "rmapi", "rmapi.conf")
		info, err := os.Stat(cfgPath)
		if os.IsNotExist(err) || (err == nil && info.Size() == 0) {
			log.Fatalf("No valid rmapi.conf detected! Please run aviary pair to complete setup. Exiting.")
			os.Exit(1)
		}
	} else {
		log.Printf("Could not determine home directory to check rmapi.conf: %v", err)
	}

	// Create a Sub FS rooted at our static export
	uiFS, err := fs.Sub(embeddedUI, "ui/out")
	if err != nil {
		log.Fatalf("embed error: %v", err)
	}

	if mode, ok := os.LookupEnv("GIN_MODE"); ok {
		gin.SetMode(mode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Gin router for /api
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// Auth endpoints (always available)
	router.POST("/api/auth/login", auth.LoginHandler)
	router.POST("/api/auth/logout", auth.LogoutHandler)
	router.GET("/api/auth/check", auth.CheckAuthHandler)

	// Protected API endpoints (require auth if configured)
	protected := router.Group("/api")
	if authRequired() {
		protected.Use(auth.ApiKeyOrJWTMiddleware())
	}

	protected.POST("/webhook", webhook.EnqueueHandler)
	protected.POST("/upload", webhook.UploadHandler)
	protected.GET("/status/:id", webhook.StatusHandler)
	router.GET("/api/config", func(c *gin.Context) {
		envUsername := os.Getenv("AUTH_USERNAME")
		envPassword := os.Getenv("AUTH_PASSWORD")
		envApiKey := os.Getenv("API_KEY")
		authEnabled := (envUsername != "" && envPassword != "") || envApiKey != ""
		c.JSON(http.StatusOK, gin.H{
			"apiUrl":     "/api/",
			"authEnabled": authEnabled,
			"apiKeyEnabled": envApiKey != "",
		})
	})

	// File server for all embedded files (gate behind AVIARY_DISABLE_UI)
	if os.Getenv("DISABLE_UI") == "" {
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
	} else {
		log.Println("DISABLE_UI is set → running in API-only mode (no UI).")
		router.NoRoute(func(c *gin.Context) {
			c.AbortWithStatus(http.StatusNotFound)
		})
	}

	log.Printf("Listening on %s…", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}

func runPair(stdout, stderr io.Writer) error {
	// 1) Are we interactive?
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("no TTY detected; please run `docker run ... aviary pair` in an interactive shell")
	}

	if host := os.Getenv("RMAPI_HOST"); host != "" {
		fmt.Fprintf(stdout, "Welcome to Aviary. Let's pair with %s!\n", host)
	} else {
		fmt.Fprintln(stdout, "Welcome to Aviary. Let's pair with the reMarkable Cloud!")
	}

	// 2) cd into rmapi
	cmd := exec.Command("rmapi", "cd")
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("`rmapi cd` failed: %w", err)
	}

	// 3) print the rmapi.conf if it exists
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not get home directory: %w", err)
	}
	cfgPath := filepath.Join(home, ".config", "rmapi", "rmapi.conf")

	fmt.Fprintf(stdout, "\nPrinting your %s file:\n", cfgPath)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("could not read config: %w", err)
	}
	stdout.Write(data)
	stdout.Write([]byte("\n"))

	fmt.Fprintln(stdout, "Done!")
	return nil
}
