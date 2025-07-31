package main

import (
	// standard library
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	// third-party
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	// internal
	"github.com/rmitchellscott/aviary/internal/auth"
	"github.com/rmitchellscott/aviary/internal/backup"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/downloader"
	"github.com/rmitchellscott/aviary/internal/handlers"
	"github.com/rmitchellscott/aviary/internal/manager"
	"github.com/rmitchellscott/aviary/internal/version"
	"github.com/rmitchellscott/aviary/internal/webhook"
)

//go:generate npm --prefix ui install
//go:generate npm --prefix ui run build
//go:embed ui/dist
//go:embed ui/dist/assets
var embeddedUI embed.FS

func main() {
	_ = godotenv.Load()
	log.Printf("[STARTUP] Starting %s", version.String())

	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(version.String())
		os.Exit(0)
	}

	if len(os.Args) > 1 && os.Args[1] == "pair" {
		if err := auth.RunPair(os.Stdout, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "pair failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if database.IsMultiUserMode() {
		if err := database.Initialize(); err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}
		defer database.Close()

		if err := database.MigrateToMultiUser(); err != nil {
			log.Fatalf("Failed to migrate to multi-user mode: %v", err)
		}

		manager.InitializeUserFolderCache(database.DB)
		
		// Start backup worker for background backup processing
		backupWorker := backup.NewWorker(database.DB)
		backupWorker.Start()
		defer backupWorker.Stop()
		log.Printf("[STARTUP] Backup worker started")
		
	}

	if err := auth.InitOIDC(); err != nil {
		log.Fatalf("Failed to initialize OIDC: %v", err)
	}

	auth.InitProxyAuth()

	port := config.Get("PORT", "")
	if port == "" {
		port = "8000"
	}
	addr := ":" + port

	// Check for rmapi.conf (skip in multi-user mode as each user will have their own config)
	// In single-user mode, just log a warning if not paired - allow UI pairing
	if !database.IsMultiUserMode() {
		if !auth.CheckSingleUserPaired() {
			log.Printf("Warning: No valid rmapi.conf detected. You can pair through the web interface or run 'aviary pair' from the command line.")
		}
	}

	// Warm the folders cache in the background so that initial page loads
	// don't block on a full directory scan. Only in single-user mode.
	if !database.IsMultiUserMode() {
		manager.StartFolderCache()
	}

	// Set up post-pairing callback to refresh folder cache
	auth.SetPostPairingCallback(func(userID string, singleUserMode bool) {
		if singleUserMode {
			if err := manager.RefreshFolderCache(); err != nil {
				log.Printf("Failed to refresh folder cache after pairing: %v", err)
			}
		} else {
			if err := manager.RefreshUserFolderCache(userID); err != nil {
				log.Printf("Failed to refresh folder cache after pairing for user %s: %v", userID, err)
			}
		}
	})

	uiFS, err := fs.Sub(embeddedUI, "ui/dist")
	if err != nil {
		log.Fatalf("embed error: %v", err)
	}

	if mode := config.Get("GIN_MODE", ""); mode != "" {
		gin.SetMode(mode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.POST("/api/auth/login", auth.MultiUserLoginHandler)
	router.POST("/api/auth/logout", auth.LogoutHandler)
	router.GET("/api/auth/check", auth.MultiUserCheckAuthHandler)
	router.GET("/api/auth/registration-status", auth.GetRegistrationStatusHandler)

	if database.IsMultiUserMode() {
		router.GET("/api/auth/oidc/login", auth.OIDCAuthHandler)
		router.GET("/api/auth/oidc/callback", auth.OIDCCallbackHandler)
		router.POST("/api/auth/oidc/logout", auth.OIDCLogoutHandler)
		router.GET("/api/auth/proxy/check", auth.ProxyAuthCheckHandler)
	}

	router.POST("/api/auth/register", auth.MultiUserAuthMiddleware(), auth.RegisterHandler)
	router.POST("/api/auth/register/public", auth.PublicRegisterHandler)
	router.POST("/api/auth/password-reset", auth.PasswordResetHandler)
	router.POST("/api/auth/password-reset/confirm", auth.PasswordResetConfirmHandler)

	protected := router.Group("/api")
	if auth.AuthRequired() || database.IsMultiUserMode() {
		protected.Use(auth.MultiUserAuthMiddleware())
	}

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

	profile := protected.Group("/profile")
	{
		profile.PUT("", auth.UpdateCurrentUserHandler)         // PUT /api/profile - update current user
		profile.POST("/password", auth.UpdatePasswordHandler)  // POST /api/profile/password - update password
		profile.POST("/pair", auth.PairRMAPIHandler)           // POST /api/profile/pair - pair rmapi
		profile.POST("/disconnect", auth.UnpairRMAPIHandler)   // POST /api/profile/disconnect - remove rmapi config
		profile.GET("/stats", auth.GetCurrentUserStatsHandler) // GET /api/profile/stats - get current user stats
		profile.DELETE("", auth.DeleteCurrentUserHandler)      // DELETE /api/profile - delete current user account
	}

	protected.POST("/pair", auth.HandlePairRequest)

	apiKeys := protected.Group("/api-keys")
	{
		apiKeys.GET("", auth.GetAPIKeysHandler)                       // GET /api/api-keys - list user's API keys
		apiKeys.POST("", auth.CreateAPIKeyHandler)                    // POST /api/api-keys - create new API key
		apiKeys.GET("/:id", auth.GetAPIKeyHandler)                    // GET /api/api-keys/:id - get specific API key
		apiKeys.PUT("/:id", auth.UpdateAPIKeyHandler)                 // PUT /api/api-keys/:id - update API key name
		apiKeys.DELETE("/:id", auth.DeleteAPIKeyHandler)              // DELETE /api/api-keys/:id - delete API key
		apiKeys.POST("/:id/deactivate", auth.DeactivateAPIKeyHandler) // POST /api/api-keys/:id/deactivate - deactivate API key
	}

	adminApiKeys := protected.Group("/admin/api-keys")
	adminApiKeys.Use(auth.AdminRequiredMiddleware())
	{
		adminApiKeys.GET("", auth.GetAllAPIKeysHandler)                  // GET /api/admin/api-keys - list all API keys
		adminApiKeys.GET("/stats", auth.GetAPIKeyStatsHandler)           // GET /api/admin/api-keys/stats - get API key stats
		adminApiKeys.POST("/cleanup", auth.CleanupExpiredAPIKeysHandler) // POST /api/admin/api-keys/cleanup - cleanup expired keys
	}

	admin := protected.Group("/admin")
	admin.Use(auth.AdminRequiredMiddleware())
	{
		admin.GET("/status", auth.GetSystemStatusHandler)        // GET /api/admin/status - get system status
		admin.GET("/settings", auth.GetSystemSettingsHandler)    // GET /api/admin/settings - get system settings
		admin.PUT("/settings", auth.UpdateSystemSettingHandler)  // PUT /api/admin/settings - update system setting
		admin.POST("/test-smtp", auth.TestSMTPHandler)           // POST /api/admin/test-smtp - test SMTP config
		admin.POST("/cleanup", auth.CleanupDataHandler)          // POST /api/admin/cleanup - cleanup old data
		admin.POST("/backup/analyze", auth.AnalyzeBackupHandler) // POST /api/admin/backup/analyze - analyze backup file
			admin.POST("/backup-job", auth.CreateBackupJobHandler)   // POST /api/admin/backup-job - create background backup job
		admin.GET("/backup-jobs", auth.GetBackupJobsHandler)     // GET /api/admin/backup-jobs - get backup jobs
		admin.GET("/backup-job/:id", auth.GetBackupJobHandler)   // GET /api/admin/backup-job/:id - get backup job
		admin.GET("/backup-job/:id/download", auth.DownloadBackupHandler) // GET /api/admin/backup-job/:id/download - download backup
		admin.DELETE("/backup-job/:id", auth.DeleteBackupJobHandler)      // DELETE /api/admin/backup-job/:id - delete backup job
		admin.POST("/restore/upload", auth.UploadRestoreFileHandler) // POST /api/admin/restore/upload - upload restore file
		admin.GET("/restore/uploads", auth.GetRestoreUploadsHandler) // GET /api/admin/restore/uploads - get pending uploads
		admin.POST("/restore/uploads/:id/analyze", auth.AnalyzeRestoreUploadHandler) // POST /api/admin/restore/uploads/:id/analyze - analyze uploaded restore file
		admin.DELETE("/restore/uploads/:id", auth.DeleteRestoreUploadHandler) // DELETE /api/admin/restore/uploads/:id - delete restore upload
		admin.POST("/restore", auth.RestoreDatabaseHandler)      // POST /api/admin/restore - restore from backup
	}

	protected.POST("/webhook", webhook.EnqueueHandler)
	protected.POST("/upload", webhook.UploadHandler)
	protected.GET("/status/:id", webhook.StatusHandler)
	protected.GET("/status/ws/:id", webhook.StatusWSHandler)
	protected.GET("/sniff", downloader.SniffHandler)
	protected.GET("/folders", manager.FoldersHandler)
	protected.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, version.Get())
	})
	router.GET("/api/config", handlers.ConfigHandler)

	if config.Get("DISABLE_UI", "") == "" {
		router.NoRoute(func(c *gin.Context) {
			p := strings.TrimPrefix(c.Request.URL.Path, "/")
			if p == "" {
				p = "index.html"
			}

			if p == "index.html" {
				envUsername := config.Get("AUTH_USERNAME", "")
				envPassword := config.Get("AUTH_PASSWORD", "")
				webAuthDisabled := envUsername == "" || envPassword == ""

				if webAuthDisabled {
					auth.ServeIndexWithSecret(c, uiFS, auth.GetUISecret())
					return
				}
			}

			if stat, err := fs.Stat(uiFS, p); err != nil || stat.IsDir() {
				p = "index.html"
				if p == "index.html" {
					envUsername := config.Get("AUTH_USERNAME", "")
					envPassword := config.Get("AUTH_PASSWORD", "")
					webAuthDisabled := envUsername == "" || envPassword == ""

					if webAuthDisabled {
						auth.ServeIndexWithSecret(c, uiFS, auth.GetUISecret())
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

	log.Printf("[STARTUP] Listening on %s…", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}
