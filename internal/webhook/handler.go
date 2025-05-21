package webhook

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary-backend/internal/compressor"
	"github.com/rmitchellscott/aviary-backend/internal/downloader"
	"github.com/rmitchellscott/aviary-backend/internal/jobs"
	"github.com/rmitchellscott/aviary-backend/internal/manager"
)

var urlRegex = regexp.MustCompile(`https?://[^\s]+`)
var jobStore = jobs.NewStore()

// EnqueueHandler accepts the form, spins off a goroutine, and returns a jobId
func EnqueueHandler(c *gin.Context) {
	// parse the same form map you had before...
	form := map[string]string{
		"Body": c.PostForm("Body"),
		// ... other fields ...
	}

	// create a job
	id := uuid.NewString()
	jobStore.Create(id)

	// process in background
	go func() {
		jobStore.Update(id, "running", "")
		msg, err := processPDF(form)
		if err != nil {
			jobStore.Update(id, "error", msg)
		} else {
			jobStore.Update(id, "success", msg)
		}
	}()

	// immediately reply with jobId
	c.JSON(http.StatusAccepted, gin.H{"jobId": id})
}

// StatusHandler returns the status & message for a given jobId
func StatusHandler(c *gin.Context) {
	id := c.Param("id")
	if job, ok := jobStore.Get(id); ok {
		c.JSON(http.StatusOK, job)
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
	}
}

func Handler(c *gin.Context) {
	// Immediate response
	c.JSON(http.StatusAccepted, gin.H{"status": "accepted"})

	// Copy form values for goroutine
	form := map[string]string{
		"Body":     c.PostForm("Body"),
		"prefix":   c.PostForm("prefix"),
		"compress": c.DefaultPostForm("compress", "false"),
		"manage":   c.DefaultPostForm("manage", "false"),
		"archive":  c.DefaultPostForm("archive", "false"),
		"rm_dir":   c.PostForm("rm_dir"),
	}
	go processPDF(form)
}

// processPDF downloads, optionally compresses, uploads/manages, and returns a status message or an error.
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

	// 1) Extract URL
	match := urlRegex.FindString(body)
	if match == "" {
		return "no URL found in request body", fmt.Errorf("no URL")
	}

	// 2) Download
	tmpDir := !archive
	manager.Logf("DownloadPDF: tmp=%t, prefix=%q", tmpDir, prefix)
	path, err := downloader.DownloadPDF(match, tmpDir, prefix)
	if err != nil {
		return "download error: " + err.Error(), err
	}

	// 3) Compress if requested
	if compress {
		manager.Logf("🔧 Compressing PDF")
		newPath, err := compressor.CompressPDF(path)
		if err != nil {
			return "compress error: " + err.Error(), err
		}
		path = newPath

		if !manage {
			// rename compressed back to original basename
			orig := strings.TrimSuffix(path, "_compressed.pdf") + ".pdf"
			if err := os.Rename(path, orig); err != nil {
				return "rename error: " + err.Error(), err
			}
			path = orig
		}
	}

	// 4) Upload / Manage flows
	switch {
	case manage && archive:
		manager.Logf("📤 Managed archive upload")
		if err := manager.RenameAndUpload(path, prefix, rmDir); err != nil {
			return "managed archive upload error: " + err.Error(), err
		}

	case manage && !archive:
		manager.Logf("📤 Managed in-place workflow")
		noYearPath, err := manager.RenameLocalNoYear(path, prefix)
		if err != nil {
			return "rename local no-year error: " + err.Error(), err
		}
		if err := manager.SimpleUpload(noYearPath, rmDir); err != nil {
			return "simple upload error: " + err.Error(), err
		}
		if _, err := manager.AppendYearLocal(noYearPath); err != nil {
			return "append-year local error: " + err.Error(), err
		}

	case !manage && archive:
		manager.Logf("📤 Archive-only upload")
		if err := manager.RenameAndUpload(path, prefix, rmDir); err != nil {
			return "archive-only upload error: " + err.Error(), err
		}

	default:
		manager.Logf("📤 Simple upload")
		if err := manager.SimpleUpload(path, rmDir); err != nil {
			return "simple upload error: " + err.Error(), err
		}
	}

	// 5) Cleanup if manage==true
	if manage {
		if err := manager.CleanupOld(prefix, rmDir); err != nil {
			manager.Logf("cleanup warning: %v", err)
		}
	}

	return fmt.Sprintf("completed workflow, final file at %q", path), nil
}

// func processPDF(form map[string]string) {
// 	body := form["Body"]
// 	prefix := form["prefix"]
// 	compress := isTrue(form["compress"])
// 	manage := isTrue(form["manage"])
// 	archive := isTrue(form["archive"])
// 	rmDir := form["rm_dir"]
// 	if rmDir == "" {
// 		rmDir = manager.DefaultRmDir()
// 	}

// 	// Extract URL
// 	match := urlRegex.FindString(body)
// 	if match == "" {
// 		manager.Logf("❌ No URL found in message")
// 		return
// 	}

// 	// 1) Download: only into PDF_DIR if archive==true
// 	tmpDir := !archive
// 	manager.Logf("DownloadPDF: tmp=%t, prefix=%q", tmpDir, prefix)
// 	path, err := downloader.DownloadPDF(match, tmpDir, prefix)
// 	if err != nil {
// 		manager.Logf("❌ Download error: %v", err)
// 		return
// 	}

// 	// 2) Compress if requested
// 	if compress {
// 		manager.Logf("🔧 Compressing PDF")
// 		path, err = compressor.CompressPDF(path)
// 		if err != nil {
// 			manager.Logf("❌ Compress error: %v", err)
// 			return
// 		}
// 		if !manage {
// 			// rename compressed back to original basename
// 			orig := strings.TrimSuffix(path, "_compressed.pdf") + ".pdf"
// 			if err := os.Rename(path, orig); err != nil {
// 				manager.Logf("❌ Rename compressed back error: %v", err)
// 				return
// 			}
// 			path = orig
// 		}
// 	}

// 	// 3) Upload / Rename & Upload
// 	switch {
// 	case manage && archive:
// 		// unchanged…
// 		manager.Logf("📤 Managed upload into PDF_DIR …")
// 		if err := manager.RenameAndUpload(path, prefix, rmDir); err != nil {
// 			manager.Logf("❌ Managed workflow error: %v", err)
// 			return
// 		}

// 	case manage && !archive:
// 		// in-place manage: split rename/upload into two steps
// 		manager.Logf("📤 Managed in-place: rename→upload(no-year)→rename(with-year) …")

// 		// 1) rename to no-year PDF
// 		noYearPath, err := manager.RenameLocalNoYear(path, prefix)
// 		if err != nil {
// 			manager.Logf("❌ Local no-year rename error: %v", err)
// 			return
// 		}

// 		// 2) upload the no-year file
// 		if err := manager.SimpleUpload(noYearPath, rmDir); err != nil {
// 			manager.Logf("❌ Upload error: %v", err)
// 			return
// 		}

// 		// 3) rename to include year
// 		if _, err := manager.AppendYearLocal(noYearPath); err != nil {
// 			manager.Logf("❌ Local append-year error: %v", err)
// 			return
// 		}

// 	case !manage && archive:
// 		manager.Logf("📤 Archive-only rename/upload into PDF_DIR …")
// 		if err := manager.RenameAndUpload(path, prefix, rmDir); err != nil {
// 			manager.Logf("❌ Archive-only workflow error: %v", err)
// 			return
// 		}

// 	default:
// 		manager.Logf("📤 Simple upload (no rename/cleanup) …")
// 		if err := manager.SimpleUpload(path, rmDir); err != nil {
// 			manager.Logf("❌ Upload error: %v", err)
// 			return
// 		}
// 	}

// 	// 4) Always cleanup when manage==true
// 	if manage {
// 		if err := manager.CleanupOld(prefix, rmDir); err != nil {
// 			manager.Logf("❌ Cleanup error: %v", err)
// 		}
// 	}
// }

func isTrue(s string) bool {
	s = strings.ToLower(s)
	return s == "true" || s == "1" || s == "yes"
}
