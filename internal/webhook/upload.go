package webhook

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/rmitchellscott/aviary/internal/i18n"
)

// UploadHandler handles a single "file" field plus optional form values.
// It saves the uploaded file under ./uploads/<original-filename>, then
// enqueues it for processing (skipping download) and returns a JSON { jobId: "..." }.
func UploadHandler(c *gin.Context) {
	// 1) Parse multipart form (allow up to 32 MiB in memory)
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.String(http.StatusBadRequest, i18n.TFromContext(c.Request.Context(), "backend.errors.parse_form")+": "+err.Error())
		return
	}

	// 2) Retrieve the *multipart.FileHeader for "file"
	fileHeader, err := getFileHeader(c.Request, "file")
	if err != nil {
		c.String(http.StatusBadRequest, fmt.Sprintf("could not get file: %v", err))
		return
	}

	// 3) Open the uploaded file
	src, err := fileHeader.Open()
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("could not open uploaded file: %v", err))
		return
	}
	defer src.Close()

	// 4) Ensure ./uploads directory exists
	uploadDir := "./uploads"
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("could not create upload dir: %v", err))
		return
	}

	// 5) Create destination file on disk
	dstPath := filepath.Join(uploadDir, filepath.Base(fileHeader.Filename))
	dstFile, err := os.Create(dstPath)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("could not create file on disk: %v", err))
		return
	}
	defer dstFile.Close()

	// 6) Copy the uploaded contents into the new file
	if _, err := io.Copy(dstFile, src); err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("could not save file: %v", err))
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
		"language": i18n.GetLanguageFromContext(c.Request.Context()),
	}

	// 9) Enqueue the job (enqueueJob is defined in handler.go)
	jobId := enqueueJob(form)

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
