package webhook

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
    "golang.org/x/text/cases"
    "golang.org/x/text/language"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/compressor"
	"github.com/rmitchellscott/aviary/internal/downloader"
	"github.com/rmitchellscott/aviary/internal/jobs"
	"github.com/rmitchellscott/aviary/internal/manager"
)

var urlRegex = regexp.MustCompile(`https?://[^\s]+`)
var jobStore = jobs.NewStore()

// EnqueueHandler accepts the form, spins off a goroutine, and returns a jobId
func EnqueueHandler(c *gin.Context) {
   form := map[string]string{
       "Body":     c.PostForm("Body"),
       "prefix":   c.PostForm("prefix"),
       "compress": c.DefaultPostForm("compress", "false"),
       "manage":   c.DefaultPostForm("manage", "false"),
       "archive":  c.DefaultPostForm("archive", "false"),
       "rm_dir":   c.PostForm("rm_dir"),
   }
    titleCaser := cases.Title(language.English)

    for key, val := range form {
        humanKey := titleCaser.String(strings.ReplaceAll(key, "_", " "))
        manager.Logf("%s: %s", humanKey, val)
    }
	// create a job
	id := uuid.NewString()
	jobStore.Create(id)

	// process in background
	go func() {
        jobStore.Update(id, "Running", "")

        // catch panics, too
        defer func() {
            if r := recover(); r != nil {
                manager.Logf("❌ Panic in processPDF: %v", r)
                jobStore.Update(id, "Error", fmt.Sprintf("Panic: %v", r))
            }
        }()

        // do the work
        msg, err := processPDF(form)
        if err != nil {
            // log the full error
            manager.Logf("❌ processPDF error: %v, message: %q", err, msg)
            jobStore.Update(id, "error", msg)
        } else {
            manager.Logf("✅ processPDF success: %s", msg)
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
	var remoteName string
	// 1) Extract URL
	match := urlRegex.FindString(body)
	if match == "" {
		return "No URL found in request body", fmt.Errorf("no URL")
	}

	// 2) Download
	tmpDir := !archive
	manager.Logf("DownloadPDF: tmp=%t, prefix=%q", tmpDir, prefix)
	path, err := downloader.DownloadPDF(match, tmpDir, prefix)
	if err != nil {
		return "Download error: " + err.Error(), err
	}

	// 3) Compress if requested
	if compress {
		manager.Logf("🔧 Compressing PDF")
		newPath, err := compressor.CompressPDF(path)
		if err != nil {
			return "Compress error: " + err.Error(), err
		}
		path = newPath

		if !manage {
			// rename compressed back to original basename
			orig := strings.TrimSuffix(path, "_compressed.pdf") + ".pdf"
			if err := os.Rename(path, orig); err != nil {
				return "Rename error: " + err.Error(), err
			}
			path = orig
		}
	}

	// 4) Upload / Manage flows
	switch {
	case manage && archive:
		manager.Logf("📤 Managed archive upload")
		remoteName, err = manager.RenameAndUpload(path, prefix, rmDir)
		if err != nil {
			return err.Error(), err
		}

	case manage && !archive:
		manager.Logf("📤 Managed in-place workflow")
		noYearPath, err := manager.RenameLocalNoYear(path, prefix)
		if err != nil {
			return err.Error(), err
		}
		remoteName, err = manager.SimpleUpload(noYearPath, rmDir)
		if err != nil {
			return err.Error(), err
		}
		if _, err := manager.AppendYearLocal(noYearPath); err != nil {
			return err.Error(), err
		}

	case !manage && archive:
		manager.Logf("📤 Archive-only upload")
		remoteName, err = manager.RenameAndUpload(path, prefix, rmDir)
		if err != nil {
			return err.Error(), err
		}

	default:
		manager.Logf("📤 Simple upload")
		remoteName, err = manager.SimpleUpload(path, rmDir)
		if err != nil {
			return err.Error(), err
		}
	}

	// 5) Cleanup if manage==true
	if manage {
		if err := manager.CleanupOld(prefix, rmDir); err != nil {
			manager.Logf("cleanup warning: %v", err)
		}
	}
	fullPath := filepath.Join(rmDir, remoteName)
	return fmt.Sprintf("✅ Your document is available on your reMarkable at %s", fullPath), nil
}

func isTrue(s string) bool {
	s = strings.ToLower(s)
	return s == "true" || s == "1" || s == "yes"
}
