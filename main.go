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
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/downloader"
	"github.com/rmitchellscott/aviary/internal/manager"
	"github.com/rmitchellscott/aviary/internal/smtp"
	"github.com/rmitchellscott/aviary/internal/webhook"
)

//go:generate npm --prefix ui install
//go:generate npm --prefix ui run build
//go:embed ui/dist
//go:embed ui/dist/assets
var embeddedUI embed.FS

// authRequired checks if API authentication is configured
func authRequired() bool {
	envApiKey := os.Getenv("API_KEY")
	return envApiKey != ""
}

func serveIndexWithSecret(c *gin.Context, uiFS fs.FS, secret string) {
	content, err := fs.ReadFile(uiFS, "index.html")
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	// Inject secret into HTML
	html := string(content)
	// Look for </head> tag and inject script before it
	scriptTag := fmt.Sprintf(`<script>window.__UI_SECRET__ = "%s";</script></head>`, secret)
	html = strings.Replace(html, "</head>", scriptTag, 1)

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, html)
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

	// Initialize database if multi-user mode is enabled
	if database.IsMultiUserMode() {
		if err := database.Initialize(); err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}
		defer database.Close()

		// Migrate from single-user to multi-user if needed
		if err := database.MigrateToMultiUser(); err != nil {
			log.Fatalf("Failed to migrate to multi-user mode: %v", err)
		}

		// Initialize user folder cache service
		manager.InitializeUserFolderCache(database.DB)
	}

	// Determine port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	addr := ":" + port

	// Check for rmapi.conf (skip in multi-user mode as each user will have their own config)
	if !database.IsMultiUserMode() {
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
	}

	// Warm the folders cache in the background so that initial page loads
	// don't block on a full directory scan.
	manager.StartFolderCache()

	// Create a Sub FS rooted at our static export
	uiFS, err := fs.Sub(embeddedUI, "ui/dist")
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
	router.POST("/api/auth/login", auth.MultiUserLoginHandler)
	router.POST("/api/auth/logout", auth.LogoutHandler)
	router.GET("/api/auth/check", auth.MultiUserCheckAuthHandler)
	router.GET("/api/auth/registration-status", auth.GetRegistrationStatusHandler) // Check if registration is enabled

	// Multi-user specific auth endpoints
	router.POST("/api/auth/register", auth.MultiUserAuthMiddleware(), auth.RegisterHandler)
	router.POST("/api/auth/register/public", auth.PublicRegisterHandler) // Public registration (when enabled)
	router.POST("/api/auth/password-reset", auth.PasswordResetHandler)
	router.POST("/api/auth/password-reset/confirm", auth.PasswordResetConfirmHandler)

	// Protected API endpoints (require auth if configured)
	protected := router.Group("/api")
	if authRequired() || database.IsMultiUserMode() {
		protected.Use(auth.MultiUserAuthMiddleware())
	}

	// Protected auth endpoints
	protected.GET("/auth/user", auth.GetCurrentUserHandler)

	// User management endpoints (multi-user mode only)
	users := protected.Group("/users")
	{
		users.GET("", auth.GetUsersHandler)                               // GET /api/users - list all users (admin)
		users.GET("/:id", auth.GetUserHandler)                            // GET /api/users/:id - get user (admin)
		users.PUT("/:id", auth.UpdateUserHandler)                         // PUT /api/users/:id - update user (admin)
		users.POST("/:id/password", auth.AdminUpdatePasswordHandler)      // POST /api/users/:id/password - update password (admin)
		users.POST("/:id/reset-password", auth.AdminResetPasswordHandler) // POST /api/users/:id/reset-password - reset password (admin)
		users.POST("/:id/deactivate", auth.DeactivateUserHandler)         // POST /api/users/:id/deactivate - deactivate user (admin)
		users.POST("/:id/activate", auth.ActivateUserHandler)             // POST /api/users/:id/activate - activate user (admin)
		users.POST("/:id/promote", auth.PromoteUserHandler)               // POST /api/users/:id/promote - promote user to admin (admin)
		users.POST("/:id/demote", auth.DemoteUserHandler)                 // POST /api/users/:id/demote - demote admin to user (admin)
		users.DELETE("/:id", auth.DeleteUserHandler)                      // DELETE /api/users/:id - delete user (admin)
		users.GET("/stats", auth.GetUserStatsHandler)                     // GET /api/users/stats - get user statistics (admin)
	}

	// Current user endpoints (multi-user mode only)
	profile := protected.Group("/profile")
	{
		profile.PUT("", auth.UpdateCurrentUserHandler)         // PUT /api/profile - update current user
		profile.POST("/password", auth.UpdatePasswordHandler)  // POST /api/profile/password - update password
		profile.POST("/pair", auth.PairRMAPIHandler)           // POST /api/profile/pair - pair rmapi
		profile.POST("/disconnect", auth.UnpairRMAPIHandler)   // POST /api/profile/disconnect - remove rmapi config
		profile.GET("/stats", auth.GetCurrentUserStatsHandler) // GET /api/profile/stats - get current user stats
		profile.DELETE("", auth.DeleteCurrentUserHandler)      // DELETE /api/profile - delete current user account
	}

	// API key management endpoints (multi-user mode only)
	apiKeys := protected.Group("/api-keys")
	{
		apiKeys.GET("", auth.GetAPIKeysHandler)                       // GET /api/api-keys - list user's API keys
		apiKeys.POST("", auth.CreateAPIKeyHandler)                    // POST /api/api-keys - create new API key
		apiKeys.GET("/:id", auth.GetAPIKeyHandler)                    // GET /api/api-keys/:id - get specific API key
		apiKeys.PUT("/:id", auth.UpdateAPIKeyHandler)                 // PUT /api/api-keys/:id - update API key name
		apiKeys.DELETE("/:id", auth.DeleteAPIKeyHandler)              // DELETE /api/api-keys/:id - delete API key
		apiKeys.POST("/:id/deactivate", auth.DeactivateAPIKeyHandler) // POST /api/api-keys/:id/deactivate - deactivate API key
	}

	// Admin API key endpoints (multi-user mode only)
	adminApiKeys := protected.Group("/admin/api-keys")
	adminApiKeys.Use(auth.AdminRequiredMiddleware())
	{
		adminApiKeys.GET("", auth.GetAllAPIKeysHandler)                  // GET /api/admin/api-keys - list all API keys
		adminApiKeys.GET("/stats", auth.GetAPIKeyStatsHandler)           // GET /api/admin/api-keys/stats - get API key stats
		adminApiKeys.POST("/cleanup", auth.CleanupExpiredAPIKeysHandler) // POST /api/admin/api-keys/cleanup - cleanup expired keys
	}

	// Admin system endpoints (multi-user mode only)
	admin := protected.Group("/admin")
	admin.Use(auth.AdminRequiredMiddleware())
	{
		admin.GET("/status", auth.GetSystemStatusHandler)       // GET /api/admin/status - get system status
		admin.GET("/settings", auth.GetSystemSettingsHandler)   // GET /api/admin/settings - get system settings
		admin.PUT("/settings", auth.UpdateSystemSettingHandler) // PUT /api/admin/settings - update system setting
		admin.POST("/test-smtp", auth.TestSMTPHandler)          // POST /api/admin/test-smtp - test SMTP config
		admin.POST("/cleanup", auth.CleanupDataHandler)         // POST /api/admin/cleanup - cleanup old data
		admin.POST("/backup", auth.BackupDatabaseHandler)       // POST /api/admin/backup - backup database
		admin.POST("/restore", auth.RestoreDatabaseHandler)     // POST /api/admin/restore - restore database
	}

	protected.POST("/webhook", webhook.EnqueueHandler)
	protected.POST("/upload", webhook.UploadHandler)
	protected.GET("/status/:id", webhook.StatusHandler)
	protected.GET("/status/ws/:id", webhook.StatusWSHandler)
	protected.GET("/sniff", downloader.SniffHandler)
	protected.GET("/folders", manager.FoldersHandler)
	router.GET("/api/config", func(c *gin.Context) {
		var authEnabled bool
		var apiKeyEnabled bool
		var multiUserMode = database.IsMultiUserMode()
		var defaultRmDir string
		var rmapiHost string

		if multiUserMode {
			// In multi-user mode, auth is always enabled
			authEnabled = true
			apiKeyEnabled = true

			// Get user-specific settings if authenticated
			if user, exists := c.Get("user"); exists {
				if dbUser, ok := user.(*database.User); ok {
					// Use user-specific defaultrmdir if set, otherwise use global default
					if dbUser.DefaultRmdir != "" {
						defaultRmDir = dbUser.DefaultRmdir
					} else {
						defaultRmDir = manager.DefaultRmDir()
					}

					// Use user-specific rmapi host if set, otherwise use global default
					if dbUser.RmapiHost != "" {
						rmapiHost = dbUser.RmapiHost
					} else {
						rmapiHost = os.Getenv("RMAPI_HOST")
					}
				} else {
					// Fallback to global defaults
					defaultRmDir = manager.DefaultRmDir()
					rmapiHost = os.Getenv("RMAPI_HOST")
				}
			} else {
				// Not authenticated, use global defaults
				defaultRmDir = manager.DefaultRmDir()
				rmapiHost = os.Getenv("RMAPI_HOST")
			}
		} else {
			// Single-user mode - use environment variables
			envUsername := os.Getenv("AUTH_USERNAME")
			envPassword := os.Getenv("AUTH_PASSWORD")
			envApiKey := os.Getenv("API_KEY")
			authEnabled = envUsername != "" && envPassword != ""
			apiKeyEnabled = envApiKey != ""
			defaultRmDir = manager.DefaultRmDir()
			rmapiHost = os.Getenv("RMAPI_HOST")
		}

		// Check SMTP configuration (only in multi-user mode)
		smtpConfigured := false
		if multiUserMode {
			smtpConfigured = smtp.IsSMTPConfigured()
		}

		c.JSON(http.StatusOK, gin.H{
			"apiUrl":         "/api/",
			"authEnabled":    authEnabled,
			"apiKeyEnabled":  apiKeyEnabled,
			"multiUserMode":  multiUserMode,
			"defaultRmDir":   defaultRmDir,
			"rmapi_host":     rmapiHost,
			"smtpConfigured": smtpConfigured,
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

			// Check if this is index.html and if we should inject UI secret
			if p == "index.html" {
				envUsername := os.Getenv("AUTH_USERNAME")
				envPassword := os.Getenv("AUTH_PASSWORD")
				webAuthDisabled := envUsername == "" || envPassword == ""

				if webAuthDisabled {
					// Web auth disabled - inject UI secret for auto-authentication
					serveIndexWithSecret(c, uiFS, auth.GetUISecret())
					return
				}
				// Web auth enabled - serve normal index.html (users must login)
			}

			// Check if file exists in embedded FS
			if stat, err := fs.Stat(uiFS, p); err != nil || stat.IsDir() {
				p = "index.html"
				// If we're falling back to index.html, check auth again
				if p == "index.html" {
					envUsername := os.Getenv("AUTH_USERNAME")
					envPassword := os.Getenv("AUTH_PASSWORD")
					webAuthDisabled := envUsername == "" || envPassword == ""

					if webAuthDisabled {
						serveIndexWithSecret(c, uiFS, auth.GetUISecret())
						return
					}
				}
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
