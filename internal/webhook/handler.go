// internal/webhook/handler.go
package webhook

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/compressor"
	"github.com/rmitchellscott/aviary/internal/converter"
	"github.com/rmitchellscott/aviary/internal/downloader"
	"github.com/rmitchellscott/aviary/internal/jobs"
	"github.com/rmitchellscott/aviary/internal/manager"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// keyToMessage converts i18n keys to human-readable messages for logging
func keyToMessage(key string) string {
	switch key {
	case "backend.status.upload_success":
		return "Upload successful"
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
	Body        string `form:"Body" json:"body"`               // URL or base64 content
	ContentType string `form:"ContentType" json:"contentType"` // MIME type for content
	Filename    string `form:"Filename" json:"filename"`       // Original filename
	IsContent   bool   `form:"IsContent" json:"isContent"`     // Flag: true=content, false=URL
	Prefix      string `form:"prefix" json:"prefix"`
	Compress    string `form:"compress" json:"compress"`
	Manage      string `form:"manage" json:"manage"`
	Archive     string `form:"archive" json:"archive"`
	RmDir       string `form:"rm_dir" json:"rm_dir"`
	RetentionDays string `form:"retention_days" json:"retention_days"`
}

// enqueueJob creates a new job ID, logs form fields, starts processPDF(form) in a goroutine,
// and returns the newly generated jobId.
func enqueueJob(form map[string]string) string {
	// Log each form field in “Human Key: Value” format.
	titleCaser := cases.Title(language.English)
	for key, val := range form {
		humanKey := titleCaser.String(strings.ReplaceAll(key, "_", " "))
		manager.Logf("%s: %s", humanKey, val)
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
				manager.Logf("❌ Panic in processPDF: %v", r)
				jobStore.Update(id, "Error", "backend.status.internal_error", nil)
			}
		}()

		// Do the actual work
		msgKey, data, err := processPDF(id, form)
		if err != nil {
			manager.Logf("❌ processPDF error: %v, message: %s", err, keyToMessage(msgKey))
			jobStore.Update(id, "error", msgKey, data)
		} else {
			logMsg := keyToMessage(msgKey)
			if data != nil && data["path"] != "" {
				logMsg += " -> " + data["path"]
			}
			manager.Logf("✅ processPDF success: %s", logMsg)
			jobStore.Update(id, "success", msgKey, data)
		}
	}()

	return id
}

// EnqueueHandler accepts either form-values (URL-based flow) or JSON (document content flow), enqueues a job, and returns JSON{"jobId": "..."}.
func EnqueueHandler(c *gin.Context) {
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
			id := enqueueDocumentJob(req)
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
			id := enqueueJob(form)
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
		}
		id := enqueueJob(form)
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
	rmDir := form["rm_dir"]
	retentionStr := form["retention_days"]
	if rmDir == "" {
		rmDir = manager.DefaultRmDir()
	}

	retentionDays := 7
	if rd, err := strconv.Atoi(retentionStr); err == nil && rd > 0 {
		retentionDays = rd
	}

	var (
		localPath  string
		remoteName string
		err        error
		cleanupErr error
	)

	// 1) If “Body” is already a valid local file path, skip download.
	if fi, statErr := os.Stat(body); statErr == nil && !fi.IsDir() {
		localPath = body
		manager.Logf("processPDF: using local file path %q, skipping download", localPath)
		jobStore.UpdateWithOperation(jobID, "Running", "backend.status.using_uploaded_file", nil, "processing")
		// Ensure we delete this file (even on error) once we're done
		defer func() {
			// Only attempt removal if the file still exists
			if _, statErr2 := os.Stat(localPath); statErr2 == nil {
				if cleanupErr = os.Remove(localPath); cleanupErr != nil {
					manager.Logf("⚠️ cleanup warning (on exit): could not remove %q: %v", localPath, cleanupErr)
				}
			}
		}()
	} else {
		// 2) Otherwise, extract a URL and download to a temp or permanent location.
		match := urlRegex.FindString(body)
		if match == "" {
			return "backend.status.no_url", nil, fmt.Errorf("no URL")
		}

		tmpDir := !archive // if archive==false, we download into a temp dir so it’ll get cleaned up
		manager.Logf("DownloadPDF: tmp=%t, prefix=%q", tmpDir, prefix)
		jobStore.UpdateWithOperation(jobID, "Running", "backend.status.downloading", nil, "downloading")
		localPath, err = downloader.DownloadPDF(match, tmpDir, prefix, nil)
		if err != nil {
			// Even if download fails, localPath may be empty—no cleanup needed here.
			return "backend.status.download_error", nil, err
		}
		// Ensure we delete this file (even on error) once we're done
		defer func() {
			// Only attempt removal if the file still exists
			if _, statErr2 := os.Stat(localPath); statErr2 == nil {
				if cleanupErr = os.Remove(localPath); cleanupErr != nil {
					manager.Logf("⚠️ cleanup warning (on exit): could not remove %q: %v", localPath, cleanupErr)
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

		// Convert the image → PDF (using PAGE_RESOLUTION & PAGE_DPI)
		pdfPath, convErr := converter.ConvertImageToPDF(origPath)
		if convErr != nil {
			return "backend.status.conversion_error", nil, convErr
		}

		// Schedule cleanup of the generated PDF at the end of processPDF
		defer func() {
			if _, statErr2 := os.Stat(pdfPath); statErr2 == nil {
				if cleanupErr = os.Remove(pdfPath); cleanupErr != nil {
					manager.Logf("⚠️ cleanup warning (on exit): could not remove generated PDF %q: %v", pdfPath, cleanupErr)
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
		if err := os.Remove(localPath); err != nil {
			manager.Logf("⚠️ failed to remove uncompressed PDF %q: %v", localPath, err)
		}

		localPath = compressedPath
		jobStore.UpdateProgress(jobID, 100)

		// If “manage” is false, rename the compressed back to “*.pdf” (drop “_compressed” suffix)
		if !manage {
			origPath := strings.TrimSuffix(localPath, "_compressed.pdf") + ".pdf"
			if err := os.Rename(localPath, origPath); err != nil {
				return "backend.status.rename_error", nil, err
			}
			localPath = origPath
		}
	}

	// 4) Upload / Manage workflows
	jobStore.UpdateWithOperation(jobID, "Running", "backend.status.uploading", nil, "uploading")
	switch {
	case manage && archive:
		manager.Logf("📤 Managed archive upload")
		remoteName, err = manager.RenameAndUpload(localPath, prefix, rmDir)
		if err != nil {
			return "backend.status.internal_error", nil, err
		}

	case manage && !archive:
		manager.Logf("📤 Managed in-place workflow")
		noYearPath, err2 := manager.RenameLocalNoYear(localPath, prefix)
		if err2 != nil {
			return "backend.status.internal_error", nil, err2
		}
		remoteName, err = manager.SimpleUpload(noYearPath, rmDir)
		if err != nil {
			return "backend.status.internal_error", nil, err
		}
		if _, err2 := manager.AppendYearLocal(noYearPath); err2 != nil {
			return "backend.status.internal_error", nil, err2
		}

	case !manage && archive:
		manager.Logf("📤 Archive-only upload")
		remoteName, err = manager.SimpleUpload(localPath, rmDir)
		if err != nil {
			return "backend.status.internal_error", nil, err
		}

	default:
		manager.Logf("📤 Simple upload")
		remoteName, err = manager.SimpleUpload(localPath, rmDir)
		if err != nil {
			return "backend.status.internal_error", nil, err
		}
	}

	// 5) If manage==true, perform cleanup
	if manage {
		if err := manager.CleanupOld(prefix, rmDir, retentionDays); err != nil {
			manager.Logf("cleanup warning: %v", err)
		}
	}

	// 6) Now that the file has been uploaded to the reMarkable, build final status message
	fullPath := filepath.Join(rmDir, remoteName)
	fullPath = strings.TrimPrefix(fullPath, "/")
	jobStore.UpdateProgress(jobID, 100)
	return "backend.status.upload_success", map[string]string{"path": fullPath}, nil
}

// enqueueDocumentJob processes document content instead of URLs
func enqueueDocumentJob(req DocumentRequest) string {
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
		msgKey, data, err := processDocument(id, req)
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
	// Create job-specific temp directory
	jobTempDir := filepath.Join(os.TempDir(), "aviary_job_"+jobID)
	if err := os.MkdirAll(jobTempDir, 0755); err != nil {
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
	
	// Create temp file in job-specific directory
	tempFile := filepath.Join(jobTempDir, filename)
	if err := os.WriteFile(tempFile, content, 0644); err != nil {
		return "backend.status.save_error", nil, fmt.Errorf("failed to save document: %w", err)
	}
	
	// Create form map for existing processing pipeline
	form := map[string]string{
		"Body":           tempFile,
		"prefix":         req.Prefix,
		"compress":       req.Compress,
		"manage":         req.Manage,
		"archive":        req.Archive,
		"rm_dir":         req.RmDir,
		"retention_days": req.RetentionDays,
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
	return processPDF(jobID, form)
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
