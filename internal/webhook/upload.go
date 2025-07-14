package webhook

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/auth"
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/manager"
)

// UploadHandler handles a single "file" field plus optional form values.
// It saves the uploaded file under per-user upload directory, then
// enqueues it for processing (skipping download) and returns a JSON { jobId: "..." }.
func UploadHandler(c *gin.Context) {
	// 1) Parse multipart form (allow up to 32 MiB in memory)
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.String(http.StatusBadRequest, "backend.errors.parse_form")
		return
	}

	// 2) Retrieve the *multipart.FileHeader for "file"
	fileHeader, err := getFileHeader(c.Request, "file")
	if err != nil {
		c.String(http.StatusBadRequest, "backend.errors.get_file")
		return
	}

	// 3) Open the uploaded file
	src, err := fileHeader.Open()
	if err != nil {
		c.String(http.StatusInternalServerError, "backend.errors.open_file")
		return
	}
	defer src.Close()

	// 4) Determine upload directory based on user context
	var uploadDir string
	var userID uuid.UUID
	
	if database.IsMultiUserMode() {
		user, ok := auth.RequireUser(c)
		if !ok {
			return // auth.RequireUser already set the response
		}
		userID = user.ID
		uploadDir, err = manager.GetUserUploadDir(userID)
		if err != nil {
			c.String(http.StatusInternalServerError, "backend.errors.create_dir")
			return
		}
	} else {
		// Single-user mode - use ./uploads
		uploadDir = "./uploads"
		if err := os.MkdirAll(uploadDir, 0o755); err != nil {
			c.String(http.StatusInternalServerError, "backend.errors.create_dir")
			return
		}
	}

	// 5) Create destination file on disk
	dstPath := filepath.Join(uploadDir, filepath.Base(fileHeader.Filename))
	dstFile, err := os.Create(dstPath)
	if err != nil {
		c.String(http.StatusInternalServerError, "backend.errors.create_file")
		return
	}
	defer dstFile.Close()

	// 6) Copy the uploaded contents into the new file
	if _, err := io.Copy(dstFile, src); err != nil {
		c.String(http.StatusInternalServerError, "backend.errors.save_file")
		return
	}

	// 7) Read additional form values (compress, manage, archive, rm_dir)
	compressVal := c.Request.FormValue("compress")
	manageVal := c.Request.FormValue("manage")
	archiveVal := c.Request.FormValue("archive")
	rmDirVal := c.Request.FormValue("rm_dir")
	prefixVal := c.Request.FormValue("prefix")

	// 8) Build the same "form" map that EnqueueHandler would use,
	//    but set Body = local path of the saved file (dstPath).
	form := map[string]string{
		"Body":     dstPath,
		"prefix":   prefixVal,
		"compress": compressVal,
		"manage":   manageVal,
		"archive":  archiveVal,
		"rm_dir":   rmDirVal,
	}

	// 9) Enqueue the job (enqueueJob is defined in handler.go)
	jobId := enqueueJobForUser(form, userID)

	// 10) Return the job ID so the client can poll /api/status/{jobId}
	c.JSON(http.StatusAccepted, gin.H{"jobId": jobId})
}

// getFileHeader retrieves the first *multipart.FileHeader under fieldName.
// Returns an error if no file is found.
func getFileHeader(r *http.Request, fieldName string) (*multipart.FileHeader, error) {
	mForm := r.MultipartForm
	if mForm == nil {
		return nil, fmt.Errorf("no multipart form found")
	}
	files := mForm.File[fieldName]
	if len(files) == 0 {
		return nil, fmt.Errorf("no file with field name %q", fieldName)
	}
	return files[0], nil
}
