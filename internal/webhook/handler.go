package webhook

import (
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rmitchellscott/aviary-backend/internal/compressor"
	"github.com/rmitchellscott/aviary-backend/internal/downloader"
	"github.com/rmitchellscott/aviary-backend/internal/manager"
)

var urlRegex = regexp.MustCompile(`https?://[^\s]+`)

// // RunServer starts the HTTP server with the webhook route
// func RunServer(addr string) error {
// 	r := gin.Default()
// 	r.POST("/webhook", func(c *gin.Context) {
// 		// Immediate response
// 		c.JSON(http.StatusAccepted, gin.H{"status": "accepted"})

//			// Copy form values for goroutine
//			form := map[string]string{
//				"Body":     c.PostForm("Body"),
//				"prefix":   c.PostForm("prefix"),
//				"compress": c.DefaultPostForm("compress", "false"),
//				"manage":   c.DefaultPostForm("manage", "false"),
//				"archive":  c.DefaultPostForm("archive", "false"),
//				"rm_dir":   c.PostForm("rm_dir"),
//			}
//			go processPDF(form)
//		})
//		return r.Run(addr)
//	}
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

func processPDF(form map[string]string) {
	body := form["Body"]
	prefix := form["prefix"]
	compress := isTrue(form["compress"])
	manage := isTrue(form["manage"])
	archive := isTrue(form["archive"])
	rmDir := form["rm_dir"]
	if rmDir == "" {
		rmDir = manager.DefaultRmDir()
	}

	// Extract URL
	match := urlRegex.FindString(body)
	if match == "" {
		manager.Logf("❌ No URL found in message")
		return
	}

	// 1) Download: only into PDF_DIR if archive==true
	tmpDir := !archive
	manager.Logf("DownloadPDF: tmp=%t, prefix=%q", tmpDir, prefix)
	path, err := downloader.DownloadPDF(match, tmpDir, prefix)
	if err != nil {
		manager.Logf("❌ Download error: %v", err)
		return
	}

	// 2) Compress if requested
	if compress {
		manager.Logf("🔧 Compressing PDF")
		path, err = compressor.CompressPDF(path)
		if err != nil {
			manager.Logf("❌ Compress error: %v", err)
			return
		}
		if !manage {
			// rename compressed back to original basename
			orig := strings.TrimSuffix(path, "_compressed.pdf") + ".pdf"
			if err := os.Rename(path, orig); err != nil {
				manager.Logf("❌ Rename compressed back error: %v", err)
				return
			}
			path = orig
		}
	}

	// 3) Upload / Rename & Upload
	switch {
	case manage && archive:
		// unchanged…
		manager.Logf("📤 Managed upload into PDF_DIR …")
		if err := manager.RenameAndUpload(path, prefix, rmDir); err != nil {
			manager.Logf("❌ Managed workflow error: %v", err)
			return
		}

	case manage && !archive:
		// in-place manage: split rename/upload into two steps
		manager.Logf("📤 Managed in-place: rename→upload(no-year)→rename(with-year) …")

		// 1) rename to no-year PDF
		noYearPath, err := manager.RenameLocalNoYear(path, prefix)
		if err != nil {
			manager.Logf("❌ Local no-year rename error: %v", err)
			return
		}

		// 2) upload the no-year file
		if err := manager.SimpleUpload(noYearPath, rmDir); err != nil {
			manager.Logf("❌ Upload error: %v", err)
			return
		}

		// 3) rename to include year
		if _, err := manager.AppendYearLocal(noYearPath); err != nil {
			manager.Logf("❌ Local append-year error: %v", err)
			return
		}

	case !manage && archive:
		manager.Logf("📤 Archive-only rename/upload into PDF_DIR …")
		if err := manager.RenameAndUpload(path, prefix, rmDir); err != nil {
			manager.Logf("❌ Archive-only workflow error: %v", err)
			return
		}

	default:
		manager.Logf("📤 Simple upload (no rename/cleanup) …")
		if err := manager.SimpleUpload(path, rmDir); err != nil {
			manager.Logf("❌ Upload error: %v", err)
			return
		}
	}

	// 4) Always cleanup when manage==true
	if manage {
		if err := manager.CleanupOld(prefix, rmDir); err != nil {
			manager.Logf("❌ Cleanup error: %v", err)
		}
	}
}

func isTrue(s string) bool {
	s = strings.ToLower(s)
	return s == "true" || s == "1" || s == "yes"
}
