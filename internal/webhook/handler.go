// internal/webhook/handler.go
package webhook

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/auth"
	"github.com/rmitchellscott/aviary/internal/compressor"
	"github.com/rmitchellscott/aviary/internal/converter"
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/downloader"
	"github.com/rmitchellscott/aviary/internal/jobs"
	"github.com/rmitchellscott/aviary/internal/manager"
	"github.com/rmitchellscott/aviary/internal/security"
	"github.com/rmitchellscott/aviary/internal/storage"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// isConflictError checks if an error is due to rmapi conflict (entry already exists)
func isConflictError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "entry already exists") || 
		   strings.Contains(errStr, "error: entry already exists")
}

// secureCleanupPaths safely removes all files in the provided path list
func secureCleanupPaths(paths []string) {
	for _, pathStr := range paths {
		if securePath, err := security.NewSecurePathFromExisting(pathStr); err == nil {
			security.SafeRemove(securePath)
		}
	}
}

// keyToMessage converts i18n keys to human-readable messages for logging
func keyToMessage(key string) string {
	switch key {
	case "backend.status.upload_success":
		return "Upload successful"
	case "backend.status.upload_success_multiple":
		return "Multiple uploads successful"
	case "backend.status.internal_error":
		return "Internal error"
	case "backend.status.using_uploaded_file":
		return "Using uploaded file"
	case "backend.status.no_url":
		return "No URL found"
	case "backend.status.downloading":
		return "Downloading"
	case "backend.status.download_error":
		return "Download error"
	case "backend.status.converting_pdf":
		return "Converting to PDF"
	case "backend.status.conversion_error":
		return "Conversion error"
	case "backend.status.compressing_pdf":
		return "Compressing PDF"
	case "backend.status.compress_error":
		return "Compression error"
	case "backend.status.rename_error":
		return "Rename error"
	case "backend.status.uploading":
		return "Uploading"
	case "backend.status.invalid_prefix":
		return "Invalid prefix"
	case "backend.status.job_not_found":
		return "Job not found"
	case "backend.status.conflict_entry_exists":
		return "Entry already exists"
	default:
		return key // fallback to key if not found
	}
}

var (
	// urlRegex is used to find an http(s) URL within the Body string.
	urlRegex = regexp.MustCompile(`https?://[^\s]+`)
	// jobStore holds in-memory jobs for status polling.
	jobStore = jobs.NewStore()
	// supportedContentTypes lists MIME types we can process
	supportedContentTypes = []string{"application/pdf", "image/jpeg", "image/png", "application/epub+zip"}
)

// DocumentRequest represents a webhook request that can contain either a URL or document content
type DocumentRequest struct {
	Body          string `form:"Body" json:"body"`               // URL or base64 content
	ContentType   string `form:"ContentType" json:"contentType"` // MIME type for content
	Filename      string `form:"Filename" json:"filename"`       // Original filename
	IsContent     bool   `form:"IsContent" json:"isContent"`     // Flag: true=content, false=URL
	Prefix        string `form:"prefix" json:"prefix"`
	Compress      string `form:"compress" json:"compress"`
	Manage        string `form:"manage" json:"manage"`
	Archive       string `form:"archive" json:"archive"`
	RmDir         string `form:"rm_dir" json:"rm_dir"`
	RetentionDays string `form:"retention_days" json:"retention_days"`
	ConflictResolution string `form:"conflict_resolution" json:"conflict_resolution"`
	Coverpage     string `form:"coverpage" json:"coverpage"`
}

// enqueueJob creates a new job ID, logs form fields, starts processPDF(form) in a goroutine,
// and returns the newly generated jobId.
func enqueueJob(form map[string]string) string {
	return enqueueJobForUser(form, uuid.Nil)
}

// enqueueJobForUser creates a new job ID for a specific user, logs form fields, starts processPDF(form) in a goroutine,
// and returns the newly generated jobId.
func enqueueJobForUser(form map[string]string, userID uuid.UUID) string {
	// Get user for logging context
	var user *database.User
	if database.IsMultiUserMode() && userID != uuid.Nil {
		userService := database.NewUserService(database.DB)
		if u, err := userService.GetUserByID(userID); err == nil {
			user = u
		}
	}

	// Log each form field in "Human Key: Value" format with username.
	titleCaser := cases.Title(language.English)
	for key, val := range form {
		humanKey := titleCaser.String(strings.ReplaceAll(key, "_", " "))
		manager.LogfWithUser(user, "%s: %s", humanKey, val)
	}

	// Create a new job in the in-memory store.
	id := uuid.NewString()
	jobStore.Create(id)

	// Launch background worker
	go func() {
		jobStore.Update(id, "Running", "", nil)
		jobStore.UpdateProgress(id, 0)

		// Catch panics
		defer func() {
			if r := recover(); r != nil {
				manager.LogfWithUser(user, "❌ Panic in processPDF: %v", r)
				jobStore.Update(id, "Error", "backend.status.internal_error", nil)
			}
		}()

		// Do the actual work
		msgKey, data, err := processPDFForUser(id, form, userID)
		if err != nil {
			manager.LogfWithUser(user, "❌ processPDF error: %v, message: %s", err, keyToMessage(msgKey))
			jobStore.Update(id, "error", msgKey, data)
		} else {
			logMsg := keyToMessage(msgKey)
			if data != nil && data["path"] != "" {
				logMsg += " -> " + data["path"]
			}
			manager.LogfWithUser(user, "✅ processPDF success: %s", logMsg)
			jobStore.Update(id, "success", msgKey, data)
		}
	}()

	return id
}

// EnqueueHandler accepts either form-values (URL-based flow) or JSON (document content flow), enqueues a job, and returns JSON{"jobId": "..."}.
func EnqueueHandler(c *gin.Context) {
	// Get user context for multi-user mode
	var userID uuid.UUID
	if database.IsMultiUserMode() {
		user, ok := auth.RequireUser(c)
		if !ok {
			return // auth.RequireUser already set the response
		}
		userID = user.ID
	}

	// Check if this is a JSON request with document content
	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(contentType, "application/json") {
		var req DocumentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
			return
		}

		// Handle document content or URL processing via JSON
		if req.IsContent {
			id := enqueueDocumentJobForUser(req, userID)
			c.JSON(http.StatusAccepted, gin.H{"jobId": id})
		} else {
			// JSON URL processing - convert to form map
			form := map[string]string{
				"Body":           req.Body,
				"prefix":         req.Prefix,
				"compress":       req.Compress,
				"manage":         req.Manage,
				"archive":        req.Archive,
				"rm_dir":         req.RmDir,
				"retention_days": req.RetentionDays,
				"conflict_resolution": req.ConflictResolution,
				"coverpage":      req.Coverpage,
			}
			// Set defaults for empty values
			if form["compress"] == "" {
				form["compress"] = "false"
			}
			if form["manage"] == "" {
				form["manage"] = "false"
			}
			if form["archive"] == "" {
				form["archive"] = "false"
			}
			if form["retention_days"] == "" {
				form["retention_days"] = "7"
			}
			id := enqueueJobForUser(form, userID)
			c.JSON(http.StatusAccepted, gin.H{"jobId": id})
		}
	} else {
		// Legacy form-encoded processing (for frontend compatibility)
		form := map[string]string{
			"Body":           c.PostForm("Body"),
			"prefix":         c.PostForm("prefix"),
			"compress":       c.DefaultPostForm("compress", "false"),
			"manage":         c.DefaultPostForm("manage", "false"),
			"archive":        c.DefaultPostForm("archive", "false"),
			"rm_dir":         c.PostForm("rm_dir"),
			"retention_days": c.DefaultPostForm("retention_days", "7"),
			"conflict_resolution": c.PostForm("conflict_resolution"),
			"coverpage":      c.PostForm("coverpage"),
		}
		id := enqueueJobForUser(form, userID)
		c.JSON(http.StatusAccepted, gin.H{"jobId": id})
	}
}

// StatusHandler returns current status & message for a given jobId.
func StatusHandler(c *gin.Context) {
	id := c.Param("id")
	if job, ok := jobStore.Get(id); ok {
		c.JSON(http.StatusOK, job)
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "backend.status.job_not_found"})
	}
}

// StatusWSHandler streams job updates over a WebSocket connection.
func StatusWSHandler(c *gin.Context) {
	id := c.Param("id")
	if _, ok := jobStore.Get(id); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "backend.status.job_not_found"})
		return
	}

	conn, err := websocket.Accept(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusInternalError, "internal error")

	ch, unsubscribe := jobStore.Subscribe(id)
	defer unsubscribe()

	ctx := c.Request.Context()
	for job := range ch {
		if err := wsjson.Write(ctx, conn, job); err != nil {
			return
		}
		if job.Status == "success" || job.Status == "error" {
			conn.Close(websocket.StatusNormalClosure, "done")
			return
		}
	}
}

// processPDF is the core pipeline: Given a form-map, it either treats form["Body"] as a local file path
// (if it exists on disk) or else extracts a URL from form["Body"], downloads it, and then proceeds to
// (optionally) compress, then upload/manage on the reMarkable. Returns a human-readable status message
// and/or an error.
func processPDF(jobID string, form map[string]string) (string, map[string]string, error) {
	return processPDFForUser(jobID, form, uuid.Nil)
}

// processPDFForUser is the core pipeline for a specific user: Given a form-map and userID, it either treats form["Body"] as a local file path
// (if it exists on disk) or else extracts a URL from form["Body"], downloads it, and then proceeds to
// (optionally) compress, then upload/manage on the reMarkable. Returns a human-readable status message
// and/or an error.
func processPDFForUser(jobID string, form map[string]string, userID uuid.UUID) (string, map[string]string, error) {

	body := form["Body"]
	prefix := form["prefix"]
	if p, perr := manager.SanitizePrefix(prefix); perr != nil {
		return "backend.status.invalid_prefix", nil, perr
	} else {
		prefix = p
	}
	compress := isTrue(form["compress"])
	manage := isTrue(form["manage"])
	archive := isTrue(form["archive"])
	retentionStr := form["retention_days"]
	requestConflictResolution := form["conflict_resolution"]
	requestCoverpage := form["coverpage"]

	retentionDays := 7
	if rd, err := strconv.Atoi(retentionStr); err == nil && rd > 0 {
		retentionDays = rd
	}

	var (
		localPath  string
		remoteName string
		err        error
		cleanupErr error
		dbUser     *database.User
	)

	if database.IsMultiUserMode() && userID != uuid.Nil {
		dbUser, _ = database.NewUserService(database.DB).GetUserByID(userID)
	}

	// Determine target reMarkable directory
	rmDir := form["rm_dir"]
	if rmDir == "" {
		// In multi-user mode, use user's personal default folder
		if database.IsMultiUserMode() && dbUser != nil && dbUser.DefaultRmdir != "" {
			rmDir = dbUser.DefaultRmdir
		} else {
			// Fallback to global default
			rmDir = manager.DefaultRmDir()
		}
	}

	// 1) Handle multiple files if Body starts with "files:"
	if strings.HasPrefix(body, "files:") {
		return processMultipleFilesForUser(jobID, form, userID)
	}

	// 2) If "Body" is already a valid local file path, skip download.
	// First validate the path to prevent path injection attacks
	if secureBodyPath, err := security.NewSecurePathFromExisting(body); err == nil {
		if fi, statErr := security.SafeStat(secureBodyPath); statErr == nil && !fi.IsDir() {
			localPath = body
			manager.Logf("processPDF: using local file path %q, skipping download", localPath)
			jobStore.UpdateWithOperation(jobID, "Running", "backend.status.using_uploaded_file", nil, "processing")
			// Ensure we delete this file (even on error) once we're done
			defer func() {
				// Only attempt removal if the file still exists
				if security.SafeStatExists(secureBodyPath) {
					if cleanupErr = security.SafeRemove(secureBodyPath); cleanupErr != nil {
						manager.Logf("⚠️ cleanup warning (on exit): could not remove %q: %v", localPath, cleanupErr)
					}
				}
			}()
		}
	} else {
		// 2) Otherwise, extract a URL and download to a temp or permanent location.
		match := urlRegex.FindString(body)
		if match == "" {
			return "backend.status.no_url", nil, fmt.Errorf("no URL")
		}

		manager.Logf("DownloadPDF: tmp=true, prefix=%q", prefix)
		jobStore.UpdateWithOperation(jobID, "Running", "backend.status.downloading", nil, "downloading")
		localPath, err = downloader.DownloadPDFForUser(match, true, prefix, userID, nil)
		if err != nil {
			// Even if download fails, localPath may be empty—no cleanup needed here.
			return "backend.status.download_error", nil, err
		}
		// Ensure we delete this file (even on error) once we're done
		defer func() {
			// Only attempt removal if the file still exists
			if secureLocalPath, pathErr := security.NewSecurePathFromExisting(localPath); pathErr == nil {
				if security.SafeStatExists(secureLocalPath) {
					if cleanupErr = security.SafeRemove(secureLocalPath); cleanupErr != nil {
						manager.Logf("⚠️ cleanup warning (on exit): could not remove %q: %v", localPath, cleanupErr)
					}
				}
			}
		}()
	}

	// 3) If the file is an image, convert it to PDF now.
	ext := strings.ToLower(filepath.Ext(localPath))
	if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
		manager.Logf("🔄 Detected image %q – converting to PDF", localPath)
		jobStore.UpdateWithOperation(jobID, "Running", "backend.status.converting_pdf", nil, "converting")
		origPath := localPath

		// Convert the image → PDF (using per-user PAGE_RESOLUTION & PAGE_DPI if available)
		var pdfPath string
		var convErr error
		if database.IsMultiUserMode() && dbUser != nil {
			// Use user-specific settings if available, falling back to environment defaults
			pdfPath, convErr = converter.ConvertImageToPDFWithSettings(origPath, dbUser.PageResolution, dbUser.PageDPI)
		} else {
			// Use environment defaults (existing behavior)
			pdfPath, convErr = converter.ConvertImageToPDF(origPath)
		}
		if convErr != nil {
			return "backend.status.conversion_error", nil, convErr
		}

		// Schedule cleanup of the generated PDF at the end of processPDF
		defer func() {
			if securePdfPath, err := security.NewSecurePathFromExisting(pdfPath); err == nil {
				if security.SafeStatExists(securePdfPath) {
					if cleanupErr = security.SafeRemove(securePdfPath); cleanupErr != nil {
						manager.Logf("⚠️ cleanup warning (on exit): could not remove generated PDF %q: %v", pdfPath, cleanupErr)
					}
				}
			}
		}()

		// Replace localPath so the rest of the pipeline uses the PDF
		localPath = pdfPath
	}

	// 4) Optionally compress the PDF
	if compress {
		manager.Logf("🔧 Compressing PDF")
		jobStore.UpdateWithOperation(jobID, "Running", "backend.status.compressing_pdf", nil, "compressing")
		jobStore.UpdateProgress(jobID, 0)
		compressedPath, compErr := compressor.CompressPDFWithProgress(localPath, func(page, total int) {
			pct := int(float64(page) / float64(total) * 100)
			jobStore.UpdateProgress(jobID, pct)
		})
		if compErr != nil {
			return "backend.status.compress_error", nil, compErr
		}

		// Remove the uncompressed version if we created a new compressed file
		if secureLocalPath, err := security.NewSecurePathFromExisting(localPath); err == nil {
			if err := security.SafeRemove(secureLocalPath); err != nil {
				manager.Logf("⚠️ failed to remove uncompressed PDF %q: %v", localPath, err)
			}
		}

		localPath = compressedPath
		jobStore.UpdateProgress(jobID, 100)

		// Always rename the compressed file back to drop "_compressed" suffix
		origPath := strings.TrimSuffix(localPath, "_compressed.pdf") + ".pdf"
		secureLocalPath, err := security.NewSecurePathFromExisting(localPath)
		if err != nil {
			return "backend.status.rename_error", nil, err
		}
		secureOrigPath, err := security.NewSecurePathFromExisting(origPath)
		if err != nil {
			return "backend.status.rename_error", nil, err
		}
		if err := security.SafeRename(secureLocalPath, secureOrigPath); err != nil {
			return "backend.status.rename_error", nil, err
		}
		localPath = origPath
	}

	// 4) Rename file for managed workflows
	var finalLocalPath string
	if manage {
		// Create new filename with month and day but no year
		today := time.Now()
		month, day := today.Format("January"), today.Day()
		
		var newFilename string
		if prefix != "" {
			manager.Logf("🔄 Renaming file for managed workflow with prefix: %s", prefix)
			newFilename = fmt.Sprintf("%s %s %d.pdf", prefix, month, day)
		} else {
			manager.Logf("🔄 Renaming file for managed workflow (no prefix)")
			newFilename = fmt.Sprintf("%s %d.pdf", month, day)
		}
		
		// Create renamed file in same directory as original
		dir := filepath.Dir(localPath)
		finalLocalPath = filepath.Join(dir, newFilename)
		
		// Copy to new location
		secureLocalPath, err := security.NewSecurePathFromExisting(localPath)
		if err != nil {
			return "backend.status.rename_error", nil, err
		}
		secureFinalPath, err := security.NewSecurePathFromExisting(finalLocalPath)
		if err != nil {
			return "backend.status.rename_error", nil, err
		}
		if err := security.SafeRename(secureLocalPath, secureFinalPath); err != nil {
			return "backend.status.rename_error", nil, err
		}
		
		// Clean up the renamed file when done
		defer func() {
			if secureFinalPath, err := security.NewSecurePathFromExisting(finalLocalPath); err == nil {
				if security.SafeStatExists(secureFinalPath) {
					if cleanupErr := security.SafeRemove(secureFinalPath); cleanupErr != nil {
						manager.Logf("⚠️ cleanup warning: could not remove renamed file %q: %v", finalLocalPath, cleanupErr)
					}
				}
			}
		}()
	} else {
		finalLocalPath = localPath
	}

	// 5) Upload to rmapi
	jobStore.UpdateWithOperation(jobID, "Running", "backend.status.uploading", nil, "uploading")
	manager.Logf("📤 Uploading to reMarkable")
	remoteName, err = manager.SimpleUpload(finalLocalPath, rmDir, dbUser, requestConflictResolution, requestCoverpage)
	if err != nil {
		if isConflictError(err) {
			return "backend.status.conflict_entry_exists", map[string]string{
				"conflict_resolution": "settings.labels.conflict_resolution",
				"settings": "app.settings",
			}, err
		}
		return "backend.status.internal_error", nil, err
	}

	// 6) Archive to storage backend if requested
	if archive {
		manager.Logf("📦 Archiving to storage backend")
		filename := filepath.Base(finalLocalPath)
		multiUserMode := database.IsMultiUserMode()
		
		var storageKey string
		if manage {
			// For managed files, use no-year format first, then add year for archival
			noYearKey := storage.GenerateUserDocumentKey(userID, prefix, filename, multiUserMode)
			
			// Copy to storage with no-year format
			ctx := context.Background()
			if err := storage.CopyFileToStorage(ctx, finalLocalPath, noYearKey); err != nil {
				manager.Logf("⚠️ archival warning: failed to copy to storage: %v", err)
			} else {
				// Add year for archival copy
				if yearKey, err2 := manager.AppendYearStorage(ctx, noYearKey, userID); err2 != nil {
					manager.Logf("⚠️ archival warning: failed to create year copy: %v", err2)
				} else {
					storageKey = yearKey
				}
			}
		} else {
			// For non-managed files, archive as-is
			storageKey = storage.GenerateUserDocumentKey(userID, "", filename, multiUserMode)
			ctx := context.Background()
			if err := storage.CopyFileToStorage(ctx, finalLocalPath, storageKey); err != nil {
				manager.Logf("⚠️ archival warning: failed to copy to storage: %v", err)
			}
		}
		
	}

	// 7) If manage==true, perform cleanup
	if manage {
		if err := manager.CleanupOld(prefix, rmDir, retentionDays, dbUser); err != nil {
			manager.Logf("cleanup warning: %v", err)
		}
	}

	// 8) Track document in database if in multi-user mode
	if database.IsMultiUserMode() && userID != uuid.Nil {
		if err := trackDocumentUpload(userID, finalLocalPath, remoteName, rmDir); err != nil {
			manager.Logf("⚠️ failed to track document upload: %v", err)
			// Continue anyway - the upload was successful
		}
	}

	// 9) Now that the file has been uploaded to the reMarkable, build final status message
	fullPath := filepath.Join(rmDir, remoteName)
	fullPath = strings.TrimPrefix(fullPath, "/")
	jobStore.UpdateProgress(jobID, 100)
	return "backend.status.upload_success", map[string]string{"path": fullPath}, nil
}

// enqueueDocumentJob processes document content instead of URLs
func enqueueDocumentJob(req DocumentRequest) string {
	return enqueueDocumentJobForUser(req, uuid.Nil)
}

// enqueueDocumentJobForUser processes document content instead of URLs for a specific user
func enqueueDocumentJobForUser(req DocumentRequest, userID uuid.UUID) string {
	// Log document processing request
	manager.Logf("Processing document content: %s (%s)", req.Filename, req.ContentType)

	// Create a new job in the in-memory store
	id := uuid.NewString()
	jobStore.Create(id)

	// Launch background worker
	go func() {
		jobStore.Update(id, "Running", "", nil)
		jobStore.UpdateProgress(id, 0)

		// Catch panics
		defer func() {
			if r := recover(); r != nil {
				manager.Logf("❌ Panic in processDocument: %v", r)
				jobStore.Update(id, "Error", "backend.status.internal_error", nil)
			}
		}()

		// Process the document content
		msgKey, data, err := processDocumentForUser(id, req, userID)
		if err != nil {
			manager.Logf("❌ processDocument error: %v, message: %q", err, msgKey)
			jobStore.Update(id, "error", msgKey, data)
		} else {
			manager.Logf("✅ processDocument success: %s", msgKey)
			jobStore.Update(id, "success", msgKey, data)
		}
	}()

	return id
}

// processDocument handles document content processing
func processDocument(jobID string, req DocumentRequest) (string, map[string]string, error) {
	return processDocumentForUser(jobID, req, uuid.Nil)
}

// processDocumentForUser handles document content processing for a specific user
func processDocumentForUser(jobID string, req DocumentRequest, userID uuid.UUID) (string, map[string]string, error) {
	// Create job-specific temp directory
	jobTempDir, err := manager.CreateUserTempDir(userID)
	if err != nil {
		return "backend.status.internal_error", nil, fmt.Errorf("failed to create job temp directory: %w", err)
	}

	// Ensure cleanup of job temp directory
	defer func() {
		if err := os.RemoveAll(jobTempDir); err != nil {
			manager.Logf("⚠️ cleanup warning: could not remove job temp directory %q: %v", jobTempDir, err)
		}
	}()

	// Validate content type if provided
	if req.ContentType != "" {
		if !isValidContentType(req.ContentType) {
			return "backend.status.unsupported_file_type", nil, fmt.Errorf("unsupported content type: %s", req.ContentType)
		}
	}

	// Decode base64 content with validation/cleaning
	jobStore.UpdateWithOperation(jobID, "Running", "backend.status.decoding_content", nil, "decoding")
	cleanedBase64 := cleanBase64(req.Body)
	content, err := base64.StdEncoding.DecodeString(cleanedBase64)
	if err != nil {
		return "backend.status.decode_error", nil, fmt.Errorf("failed to decode document content: %w", err)
	}

	// Verify content type matches actual content
	detectedType := http.DetectContentType(content[:min(512, len(content))])
	if req.ContentType != "" && !isContentTypeMatch(req.ContentType, detectedType) {
		manager.Logf("⚠️ Content type mismatch: claimed %s, detected %s", req.ContentType, detectedType)
		// Continue but log the mismatch - use detected type for processing
	}

	// Sanitize filename
	filename := sanitizeFilename(req.Filename)
	if filename == "" {
		filename = "document"
		// Add extension based on detected or claimed content type
		contentTypeToUse := req.ContentType
		if contentTypeToUse == "" {
			contentTypeToUse = detectedType
		}
		switch {
		case strings.HasPrefix(contentTypeToUse, "application/pdf"):
			filename += ".pdf"
		case strings.HasPrefix(contentTypeToUse, "image/jpeg"):
			filename += ".jpg"
		case strings.HasPrefix(contentTypeToUse, "image/png"):
			filename += ".png"
		case strings.HasPrefix(contentTypeToUse, "application/epub"):
			filename += ".epub"
		}
	}

	// Create temp file with proper extension
	tempFile, err := ioutil.TempFile("", "aviary-doc-*"+filepath.Ext(filename))
	if err != nil {
		return "backend.status.save_error", nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFilePath := tempFile.Name()
	
	// Write content to temp file
	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close()
		os.Remove(tempFilePath)
		return "backend.status.save_error", nil, fmt.Errorf("failed to write document: %w", err)
	}
	tempFile.Close()

	// Create form map for existing processing pipeline
	form := map[string]string{
		"Body":           tempFilePath,
		"prefix":         req.Prefix,
		"compress":       req.Compress,
		"manage":         req.Manage,
		"archive":        req.Archive,
		"rm_dir":         req.RmDir,
		"retention_days": req.RetentionDays,
		"conflict_resolution": req.ConflictResolution,
		"coverpage":      req.Coverpage,
	}

	// Set defaults for empty values
	if form["compress"] == "" {
		form["compress"] = "false"
	}
	if form["manage"] == "" {
		form["manage"] = "false"
	}
	if form["archive"] == "" {
		form["archive"] = "false"
	}
	if form["retention_days"] == "" {
		form["retention_days"] = "7"
	}

	// Process through existing pipeline
	return processPDFForUser(jobID, form, userID)
}

// isValidContentType checks if the content type is supported
func isValidContentType(contentType string) bool {
	for _, supported := range supportedContentTypes {
		if strings.HasPrefix(contentType, supported) {
			return true
		}
	}
	return false
}

// cleanBase64 removes whitespace and validates base64 content
func cleanBase64(data string) string {
	// Remove all whitespace (spaces, newlines, tabs)
	cleaned := strings.ReplaceAll(data, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "\n", "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	cleaned = strings.ReplaceAll(cleaned, "\t", "")
	return cleaned
}

// isContentTypeMatch checks if claimed content type matches detected type
func isContentTypeMatch(claimed, detected string) bool {
	// Handle common variations
	switch {
	case strings.HasPrefix(claimed, "application/pdf") && strings.HasPrefix(detected, "application/pdf"):
		return true
	case strings.HasPrefix(claimed, "image/jpeg") && strings.HasPrefix(detected, "image/jpeg"):
		return true
	case strings.HasPrefix(claimed, "image/png") && strings.HasPrefix(detected, "image/png"):
		return true
	case strings.HasPrefix(claimed, "application/epub") && strings.HasPrefix(detected, "application/zip"):
		// EPUB files are detected as application/zip
		return true
	default:
		return strings.HasPrefix(detected, claimed) || strings.HasPrefix(claimed, detected)
	}
}

// sanitizeFilename removes dangerous characters from filenames
func sanitizeFilename(filename string) string {
	if filename == "" {
		return ""
	}

	// Use only the base filename (no path traversal)
	filename = filepath.Base(filename)

	// Remove dangerous characters
	dangerous := []string{"..", "~", "$", "`", "|", ";", "&", "<", ">", "(", ")", "{", "}", "[", "]", "'", `"`}
	for _, char := range dangerous {
		filename = strings.ReplaceAll(filename, char, "_")
	}

	// Limit length
	if len(filename) > 200 {
		ext := filepath.Ext(filename)
		base := filename[:200-len(ext)]
		filename = base + ext
	}

	return filename
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// isTrue interprets "true"/"1"/"yes" (case-insensitive) as true.
func isTrue(s string) bool {
	s = strings.ToLower(s)
	return s == "true" || s == "1" || s == "yes"
}

// trackDocumentUpload records a document upload in the database
func trackDocumentUpload(userID uuid.UUID, localPath, remoteName, rmDir string) error {
	if database.DB == nil {
		return nil // Database not initialized
	}

	// Get file info
	var fileSize int64
	if secureLocalPath, err := security.NewSecurePathFromExisting(localPath); err == nil {
		if info, err := security.SafeStat(secureLocalPath); err == nil {
			fileSize = info.Size()
		}
	}

	// Determine document type from extension
	ext := strings.ToLower(filepath.Ext(localPath))
	var docType string
	switch ext {
	case ".pdf":
		docType = "PDF"
	case ".jpg", ".jpeg":
		docType = "JPEG"
	case ".png":
		docType = "PNG"
	case ".epub":
		docType = "EPUB"
	default:
		docType = "Unknown"
	}

	// Create document record
	doc := database.Document{
		ID:           uuid.New(),
		UserID:       userID,
		DocumentName: remoteName,
		LocalPath:    localPath,
		RemotePath:   filepath.Join(rmDir, remoteName),
		DocumentType: docType,
		FileSize:     fileSize,
		Status:       "uploaded",
	}

	return database.DB.Create(&doc).Error
}

// processMultipleFilesForUser handles processing multiple files uploaded together
func processMultipleFilesForUser(jobID string, form map[string]string, userID uuid.UUID) (string, map[string]string, error) {

	body := form["Body"]
	compress := isTrue(form["compress"])
	requestConflictResolution := form["conflict_resolution"]
	requestCoverpage := form["coverpage"]

	// Extract file paths from the "files:" prefix
	pathsJSON := strings.TrimPrefix(body, "files:")
	var filePaths []string
	if err := json.Unmarshal([]byte(pathsJSON), &filePaths); err != nil {
		return "backend.status.internal_error", nil, fmt.Errorf("failed to parse file paths: %w", err)
	}
	
	// Validate all file paths to prevent path injection attacks
	for i, filePath := range filePaths {
		if err := security.ValidateFilePath(filePath); err != nil {
			return "backend.status.internal_error", nil, fmt.Errorf("invalid file path at index %d: %w", i, err)
		}
	}

	var (
		dbUser         *database.User
		finalPaths     []string
		totalPages     int
		processedPages int
		cleanupPaths   []string
	)

	if database.IsMultiUserMode() && userID != uuid.Nil {
		dbUser, _ = database.NewUserService(database.DB).GetUserByID(userID)
	}

	// Determine target reMarkable directory
	rmDir := form["rm_dir"]
	if rmDir == "" {
		if database.IsMultiUserMode() && dbUser != nil && dbUser.DefaultRmdir != "" {
			rmDir = dbUser.DefaultRmdir
		} else {
			rmDir = manager.DefaultRmDir()
		}
	}

	jobStore.UpdateWithOperation(jobID, "Running", "backend.status.using_uploaded_file", nil, "processing")

	// Separate files by type and process compressible files first, then EPUBs
	var compressibleFiles []string
	var epubFiles []string
	
	for _, filePath := range filePaths {
		ext := strings.ToLower(filepath.Ext(filePath))
		if ext == ".epub" {
			epubFiles = append(epubFiles, filePath)
		} else {
			compressibleFiles = append(compressibleFiles, filePath)
		}
	}
	
	// Reorder file paths: compressible files first, then EPUBs
	orderedPaths := append(compressibleFiles, epubFiles...)

	// Pre-calculate total pages for accurate progress tracking (only for compressible files)
	if compress {
		for _, filePath := range compressibleFiles {
			ext := strings.ToLower(filepath.Ext(filePath))
			if ext == ".pdf" {
				if pages, err := compressor.GetPDFPageCount(filePath); err == nil {
					totalPages += pages
				} else {
					// Fallback: assume 1 page if we can't get count
					totalPages += 1
				}
			} else if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
				totalPages += 1 // Images become 1-page PDFs
			}
		}
		// Set compression status once for the entire batch
		if totalPages > 0 {
			jobStore.UpdateWithOperation(jobID, "Running", "backend.status.compressing_pdf", nil, "compressing")
			jobStore.UpdateProgress(jobID, 0)
		}
	}

	// Process each file in the reordered list
	for _, filePath := range orderedPaths {
		cleanupPaths = append(cleanupPaths, filePath)

		// Convert images to PDF if needed
		ext := strings.ToLower(filepath.Ext(filePath))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
			manager.Logf("🔄 Converting image %q to PDF", filePath)
			var pdfPath string
			var convErr error
			if database.IsMultiUserMode() && dbUser != nil {
				pdfPath, convErr = converter.ConvertImageToPDFWithSettings(filePath, dbUser.PageResolution, dbUser.PageDPI)
			} else {
				pdfPath, convErr = converter.ConvertImageToPDF(filePath)
			}
			if convErr != nil {
				// Clean up any processed files before returning error
				secureCleanupPaths(cleanupPaths)
				return "backend.status.conversion_error", nil, convErr
			}
			cleanupPaths = append(cleanupPaths, pdfPath)
			filePath = pdfPath
		}

		// Compress PDF if requested and file is PDF
		if compress && (strings.ToLower(filepath.Ext(filePath)) == ".pdf") {
			manager.Logf("🔧 Compressing PDF %q", filePath)

			// Get the expected page count for this file from our pre-calculation
			expectedPages := 1 // fallback
			if pages, err := compressor.GetPDFPageCount(filePath); err == nil {
				expectedPages = pages
			}

			compressedPath, compErr := compressor.CompressPDFWithProgress(filePath, func(page, total int) {
				// Calculate overall progress: (all previously processed pages + current progress within this file) / total pages
				if totalPages > 0 && total > 0 {
					// Current progress within this file as a fraction of its total pages
					fileProgressPages := float64(page*expectedPages) / float64(total)
					overallProgress := int((float64(processedPages) + fileProgressPages) / float64(totalPages) * 100)
					jobStore.UpdateProgress(jobID, overallProgress)
				}
			})
			if compErr != nil {
				// Clean up any processed files before returning error
				secureCleanupPaths(cleanupPaths)
				return "backend.status.compress_error", nil, compErr
			}

			// Update processed pages count with the expected pages for this file
			processedPages += expectedPages

			// Remove uncompressed version
			if secureFilePath, err := security.NewSecurePathFromExisting(filePath); err == nil {
				security.SafeRemove(secureFilePath)
			}
			cleanupPaths = append(cleanupPaths, compressedPath)
			
			// Rename compressed file to remove "_compressed" suffix (to match single file flow)
			origPath := strings.TrimSuffix(compressedPath, "_compressed.pdf") + ".pdf"
			secureCompressedPath, compErr := security.NewSecurePathFromExisting(compressedPath)
			secureOrigPath, origErr := security.NewSecurePathFromExisting(origPath)
			if compErr != nil || origErr != nil || security.SafeRename(secureCompressedPath, secureOrigPath) != nil {
				// Clean up any processed files before returning error
				secureCleanupPaths(cleanupPaths)
				return "backend.status.rename_error", nil, fmt.Errorf("failed to rename compressed file")
			}
			
			// Update cleanup paths and file path
			cleanupPaths[len(cleanupPaths)-1] = origPath // Replace the last entry
			filePath = origPath
		} else if compress {
			// For non-PDF files (images), still increment processed pages to keep progress accurate
			processedPages += 1
		}

		finalPaths = append(finalPaths, filePath)
	}

	// Upload all processed files
	var uploadedPaths []string
	jobStore.UpdateWithOperation(jobID, "Running", "backend.status.uploading", nil, "uploading")

	for _, filePath := range finalPaths {
		// Use simple upload for each file
		remoteName, err := manager.SimpleUpload(filePath, rmDir, dbUser, requestConflictResolution, requestCoverpage)
		if err != nil {
			// Clean up any processed files before returning error
			secureCleanupPaths(cleanupPaths)
			if isConflictError(err) {
				return "backend.status.conflict_entry_exists", map[string]string{
					"conflict_resolution": "settings.labels.conflict_resolution",
					"settings": "app.settings",
				}, err
			}
			return "backend.status.internal_error", nil, err
		}

		fullPath := filepath.Join(rmDir, remoteName)
		fullPath = strings.TrimPrefix(fullPath, "/")
		uploadedPaths = append(uploadedPaths, fullPath)

		// Track document in database if in multi-user mode
		if database.IsMultiUserMode() && userID != uuid.Nil {
			if err := trackDocumentUpload(userID, filePath, remoteName, rmDir); err != nil {
				manager.Logf("⚠️ failed to track document upload: %v", err)
			}
		}
	}

	for _, path := range cleanupPaths {
		if securePath, err := security.NewSecurePathFromExisting(path); err == nil {
			if security.SafeStatExists(securePath) {
				if err := security.SafeRemove(securePath); err != nil {
					manager.Logf("⚠️ cleanup warning: could not remove %q: %v", path, err)
				}
			}
		}
	}

	// Return success with all uploaded paths as JSON array for proper HTML list rendering
	pathsJSONBytes, err := json.Marshal(uploadedPaths)
	if err != nil {
		// Fallback to simple join if JSON marshaling fails
		return "backend.status.upload_success_multiple", map[string]string{"paths": strings.Join(uploadedPaths, "\n")}, nil
	}

	jobStore.UpdateProgress(jobID, 100)
	return "backend.status.upload_success_multiple", map[string]string{"paths": string(pathsJSONBytes)}, nil
}
