package webhook

import (
	"encoding/json"
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
// It saves the uploaded file to temporary storage, then
// enqueues it for processing (skipping download) and returns a JSON { jobId: "..." }.
func UploadHandler(c *gin.Context) {
	// 1) Parse multipart form (allow up to 32 MiB in memory)
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.String(http.StatusBadRequest, "backend.errors.parse_form")
		return
	}

	// 2) Retrieve file(s) - try "files" first for multiple, then "file" for single
	var fileHeaders []*multipart.FileHeader
	var err error
	
	// Try multiple files first
	if files := c.Request.MultipartForm.File["files"]; len(files) > 0 {
		fileHeaders = files
	} else {
		// Fall back to single file
		fileHeader, err := getFileHeader(c.Request, "file")
		if err != nil {
			c.String(http.StatusBadRequest, "backend.errors.get_file")
			return
		}
		fileHeaders = []*multipart.FileHeader{fileHeader}
	}

	// 3) Determine upload directory based on user context (always use temp for uploads)
	var uploadDir string
	var userID uuid.UUID
	
	if database.IsMultiUserMode() {
		user, ok := auth.RequireUser(c)
		if !ok {
			return // auth.RequireUser already set the response
		}
		userID = user.ID
	}
	
	uploadDir, err = manager.CreateUserTempDir(userID)
	if err != nil {
		c.String(http.StatusInternalServerError, "backend.errors.create_dir")
		return
	}

	// 4) Process each file and save to temp filesystem location
	var savedPaths []string
	for _, fileHeader := range fileHeaders {
		// Open the uploaded file
		src, err := fileHeader.Open()
		if err != nil {
			c.String(http.StatusInternalServerError, "backend.errors.open_file")
			return
		}
		defer src.Close()

		// Create destination file in temp location
		dstPath := filepath.Join(uploadDir, filepath.Base(fileHeader.Filename))
		dstFile, err := os.Create(dstPath)
		if err != nil {
			c.String(http.StatusInternalServerError, "backend.errors.create_file")
			return
		}
		defer dstFile.Close()

		// Copy the uploaded contents into the new file
		if _, err := io.Copy(dstFile, src); err != nil {
			c.String(http.StatusInternalServerError, "backend.errors.save_file")
			return
		}
		
		savedPaths = append(savedPaths, dstPath)
	}

	// 5) Read additional form values (compress, manage, archive, rm_dir)
	compressVal := c.Request.FormValue("compress")
	manageVal := c.Request.FormValue("manage")
	archiveVal := c.Request.FormValue("archive")
	rmDirVal := c.Request.FormValue("rm_dir")
	prefixVal := c.Request.FormValue("prefix")

	// 6) For multiple files, create a single job with all file paths
	var jobId string
	if len(savedPaths) == 1 {
		// Single file - use existing logic
		form := map[string]string{
			"Body":     savedPaths[0],
			"prefix":   prefixVal,
			"compress": compressVal,
			"manage":   manageVal,
			"archive":  archiveVal,
			"rm_dir":   rmDirVal,
		}
		jobId = enqueueJobForUser(form, userID)
	} else {
		// Multiple files - create job with JSON-encoded paths
		pathsJSON, err := json.Marshal(savedPaths)
		if err != nil {
			c.String(http.StatusInternalServerError, "backend.errors.internal_error")
			return
		}
		form := map[string]string{
			"Body":     fmt.Sprintf("files:%s", string(pathsJSON)),
			"prefix":   prefixVal,
			"compress": compressVal,
			"manage":   manageVal,
			"archive":  archiveVal,
			"rm_dir":   rmDirVal,
		}
		jobId = enqueueJobForUser(form, userID)
	}

	// 7) Return the job ID so the client can poll /api/status/{jobId}
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
