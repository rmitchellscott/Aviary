// internal/webhook/handler.go
package webhook

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/compressor"
	"github.com/rmitchellscott/aviary/internal/downloader"
	"github.com/rmitchellscott/aviary/internal/jobs"
	"github.com/rmitchellscott/aviary/internal/manager"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	// urlRegex is used to find an http(s) URL within the Body string.
	urlRegex = regexp.MustCompile(`https?://[^\s]+`)
	// jobStore holds in-memory jobs for status polling.
	jobStore = jobs.NewStore()
)

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
		jobStore.Update(id, "Running", "")

		// Catch panics
		defer func() {
			if r := recover(); r != nil {
				manager.Logf("❌ Panic in processPDF: %v", r)
				jobStore.Update(id, "Error", fmt.Sprintf("Panic: %v", r))
			}
		}()

		// Do the actual work
		msg, err := processPDF(form)
		if err != nil {
			manager.Logf("❌ processPDF error: %v, message: %q", err, msg)
			jobStore.Update(id, "error", msg)
		} else {
			manager.Logf("✅ processPDF success: %s", msg)
			jobStore.Update(id, "success", msg)
		}
	}()

	return id
}

// EnqueueHandler accepts form-values (URL-based flow), enqueues a job, and returns JSON{"jobId": "..."}.
func EnqueueHandler(c *gin.Context) {
	form := map[string]string{
		"Body":     c.PostForm("Body"),
		"prefix":   c.PostForm("prefix"),
		"compress": c.DefaultPostForm("compress", "false"),
		"manage":   c.DefaultPostForm("manage", "false"),
		"archive":  c.DefaultPostForm("archive", "false"),
		"rm_dir":   c.PostForm("rm_dir"),
	}
	id := enqueueJob(form)
	c.JSON(http.StatusAccepted, gin.H{"jobId": id})
}

// StatusHandler returns current status & message for a given jobId.
func StatusHandler(c *gin.Context) {
	id := c.Param("id")
	if job, ok := jobStore.Get(id); ok {
		c.JSON(http.StatusOK, job)
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
	}
}

// processPDF is the core pipeline: Given a form-map, it either treats form["Body"] as a local file path
// (if it exists on disk) or else extracts a URL from form["Body"], downloads it, and then proceeds to
// (optionally) compress, then upload/manage on the reMarkable. Returns a human-readable status message
// and/or an error.
func processPDF(form map[string]string) (string, error) {
	body := form["Body"]
	prefix := form["prefix"]
	compress := isTrue(form["compress"])
	manage := isTrue(form["manage"])
	archive := isTrue(form["archive"])
	rmDir := form["rm_dir"]
	if rmDir == "" {
		rmDir = manager.DefaultRmDir()
	}

	var (
		localPath  string
		remoteName string
		err        error
	)

	// 1) If “Body” is already a valid local file path, skip download.
	if fi, statErr := os.Stat(body); statErr == nil && !fi.IsDir() {
		localPath = body
		manager.Logf("processPDF: using local file path %q, skipping download", localPath)
	} else {
		// 2) Otherwise, extract a URL and download to a temp or permanent location.
		match := urlRegex.FindString(body)
		if match == "" {
			return "No URL found in request body", fmt.Errorf("no URL")
		}

		tmpDir := !archive // if archive==false, we download into a temp dir so it’ll get cleaned up
		manager.Logf("DownloadPDF: tmp=%t, prefix=%q", tmpDir, prefix)
		localPath, err = downloader.DownloadPDF(match, tmpDir, prefix)
		if err != nil {
			return "Download error: " + err.Error(), err
		}
	}

	// 3) Optionally compress the PDF
	if compress {
		manager.Logf("🔧 Compressing PDF")
		compressedPath, compErr := compressor.CompressPDF(localPath)
		if compErr != nil {
			return "Compress error: " + compErr.Error(), compErr
		}

		// Remove the uncompressed version if we created a new compressed file
		if err := os.Remove(localPath); err != nil {
			manager.Logf("⚠️ failed to remove uncompressed PDF %q: %v", localPath, err)
		}

		localPath = compressedPath

		// If “manage” is false, rename the compressed back to “*.pdf” (drop “_compressed” suffix)
		if !manage {
			origPath := strings.TrimSuffix(localPath, "_compressed.pdf") + ".pdf"
			if err := os.Rename(localPath, origPath); err != nil {
				return "Rename error: " + err.Error(), err
			}
			localPath = origPath
		}
	}

	// 4) Upload / Manage workflows
	switch {
	case manage && archive:
		manager.Logf("📤 Managed archive upload")
		remoteName, err = manager.RenameAndUpload(localPath, prefix, rmDir)
		if err != nil {
			return err.Error(), err
		}

	case manage && !archive:
		manager.Logf("📤 Managed in-place workflow")
		noYearPath, err2 := manager.RenameLocalNoYear(localPath, prefix)
		if err2 != nil {
			return err2.Error(), err2
		}
		remoteName, err = manager.SimpleUpload(noYearPath, rmDir)
		if err != nil {
			return err.Error(), err
		}
		if _, err2 := manager.AppendYearLocal(noYearPath); err2 != nil {
			return err2.Error(), err2
		}

	case !manage && archive:
		manager.Logf("📤 Archive-only upload")
		remoteName, err = manager.SimpleUpload(localPath, rmDir)
		if err != nil {
			return err.Error(), err
		}

	default:
		manager.Logf("📤 Simple upload")
		remoteName, err = manager.SimpleUpload(localPath, rmDir)
		if err != nil {
			return err.Error(), err
		}
	}

	// 5) If manage==true, perform cleanup
	if manage {
		if err := manager.CleanupOld(prefix, rmDir); err != nil {
			manager.Logf("cleanup warning: %v", err)
		}
	}

	// 6) Build a final status message pointing to the reMarkable path
	fullPath := filepath.Join(rmDir, remoteName)
	return fmt.Sprintf("✅ Your document is available on your reMarkable at %s", fullPath), nil
}

// isTrue interprets "true"/"1"/"yes" (case-insensitive) as true.
func isTrue(s string) bool {
	s = strings.ToLower(s)
	return s == "true" || s == "1" || s == "yes"
}
