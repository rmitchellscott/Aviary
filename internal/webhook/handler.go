// internal/webhook/handler.go
package webhook

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

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
		jobStore.UpdateProgress(id, 0)

		// Catch panics
		defer func() {
			if r := recover(); r != nil {
				manager.Logf("❌ Panic in processPDF: %v", r)
				jobStore.Update(id, "Error", fmt.Sprintf("Panic: %v", r))
			}
		}()

		// Do the actual work
		msg, err := processPDF(id, form)
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
func processPDF(jobID string, form map[string]string) (string, error) {
	body := form["Body"]
	prefix := form["prefix"]
	if p, perr := manager.SanitizePrefix(prefix); perr != nil {
		return "invalid prefix", perr
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
		jobStore.Update(jobID, "Running", "Using uploaded file")
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
			return "No URL found in request body", fmt.Errorf("no URL")
		}

		tmpDir := !archive // if archive==false, we download into a temp dir so it’ll get cleaned up
		manager.Logf("DownloadPDF: tmp=%t, prefix=%q", tmpDir, prefix)
		jobStore.Update(jobID, "Running", "Downloading")
		localPath, err = downloader.DownloadPDF(match, tmpDir, prefix, nil)
		if err != nil {
			// Even if download fails, localPath may be empty—no cleanup needed here.
			return "Download error: " + err.Error(), err
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
		jobStore.Update(jobID, "Running", "Converting to PDF")
		origPath := localPath

		// Convert the image → PDF (using PAGE_RESOLUTION & PAGE_DPI)
		pdfPath, convErr := converter.ConvertImageToPDF(origPath)
		if convErr != nil {
			return "Image→PDF conversion error: " + convErr.Error(), convErr
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
		jobStore.Update(jobID, "Running", "Compressing PDF")
		jobStore.UpdateProgress(jobID, 0)
		compressedPath, compErr := compressor.CompressPDFWithProgress(localPath, func(page, total int) {
			pct := int(float64(page) / float64(total) * 100)
			jobStore.UpdateProgress(jobID, pct)
		})
		if compErr != nil {
			return "Compress error: " + compErr.Error(), compErr
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
				return "Rename error: " + err.Error(), err
			}
			localPath = origPath
		}
	}

	// 4) Upload / Manage workflows
	jobStore.Update(jobID, "Running", "Uploading")
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
		if err := manager.CleanupOld(prefix, rmDir, retentionDays); err != nil {
			manager.Logf("cleanup warning: %v", err)
		}
	}

	// 6) Now that the file has been uploaded to the reMarkable, build final status message
	fullPath := filepath.Join(rmDir, remoteName)
	fullPath = strings.TrimPrefix(fullPath, "/")
	jobStore.UpdateProgress(jobID, 100)
	return fmt.Sprintf("Your document is available on your reMarkable at %s", fullPath), nil
}

// isTrue interprets "true"/"1"/"yes" (case-insensitive) as true.
func isTrue(s string) bool {
	s = strings.ToLower(s)
	return s == "true" || s == "1" || s == "yes"
}
