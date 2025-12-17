package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/auth"
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/logging"
	"github.com/rmitchellscott/aviary/internal/manager"
)

func UploadHandler(c *gin.Context) {
	maxUploadSize := getMaxUploadSize()
	
	contentLength := c.Request.ContentLength
	if contentLength > maxUploadSize {
		logging.Logf("[UPLOAD] File too large: %d bytes (limit: %d bytes)", contentLength, maxUploadSize)
		c.String(http.StatusBadRequest, "backend.errors.file_too_large")
		return
	}

	mediaType, params, err := parseContentType(c.Request.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		c.String(http.StatusBadRequest, "backend.errors.parse_form")
		return
	}
	
	boundary := params["boundary"]
	if boundary == "" {
		c.String(http.StatusBadRequest, "backend.errors.parse_form")
		return
	}
	
	multipartReader := multipart.NewReader(c.Request.Body, boundary)

	var userID uuid.UUID
	if database.IsMultiUserMode() {
		user, ok := auth.RequireUser(c)
		if !ok {
			return
		}
		userID = user.ID
	}

	// 3) Process multipart stream to extract files and form values
	var savedPaths []string
	var formValues = make(map[string]string)
	
	for {
		part, err := multipartReader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			c.String(http.StatusBadRequest, "backend.errors.parse_form")
			return
		}
		
		fieldName := part.FormName()
		filename := part.FileName()
		
		if filename != "" {
			if fieldName == "file" || fieldName == "files" {
				filePath, err := processFilePart(part, filename, userID)
				if err != nil {
					logging.Logf("[UPLOAD] Failed to process file %s: %v", filename, err)
					c.String(http.StatusInternalServerError, "backend.errors.upload_stream_failed")
					return
				}
				savedPaths = append(savedPaths, filePath)
				logging.Logf("[UPLOAD] Successfully processed file: %s", filename)
			}
		} else {
			value, err := io.ReadAll(part)
			if err != nil {
				c.String(http.StatusBadRequest, "backend.errors.parse_form")
				return
			}
			formValues[fieldName] = string(value)
		}
		
		part.Close()
	}
	
	if len(savedPaths) == 0 {
		c.String(http.StatusBadRequest, "backend.errors.get_file")
		return
	}

	compressVal := formValues["compress"]
	manageVal := formValues["manage"]
	archiveVal := formValues["archive"]
	rmDirVal := formValues["rm_dir"]
	prefixVal := formValues["prefix"]
	removeBackgroundVal := formValues["remove_background"]
	var jobId string
	if len(savedPaths) == 1 {
		form := map[string]string{
			"Body":              savedPaths[0],
			"prefix":            prefixVal,
			"compress":          compressVal,
			"manage":            manageVal,
			"archive":           archiveVal,
			"rm_dir":            rmDirVal,
			"remove_background": removeBackgroundVal,
		}
		jobId = enqueueJobForUser(form, userID)
	} else {
		pathsJSON, err := json.Marshal(savedPaths)
		if err != nil {
			c.String(http.StatusInternalServerError, "backend.errors.internal_error")
			return
		}
		form := map[string]string{
			"Body":              fmt.Sprintf("files:%s", string(pathsJSON)),
			"prefix":            prefixVal,
			"compress":          compressVal,
			"manage":            manageVal,
			"archive":           archiveVal,
			"rm_dir":            rmDirVal,
			"remove_background": removeBackgroundVal,
		}
		jobId = enqueueJobForUser(form, userID)
	}
	c.JSON(http.StatusAccepted, gin.H{"jobId": jobId})
}

func getMaxUploadSize() int64 {
	if sizeStr := os.Getenv("MAX_UPLOAD_SIZE"); sizeStr != "" {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
			return size
		}
	}
	return 500 << 20
}

func parseContentType(contentType string) (mediaType string, params map[string]string, err error) {
	parts := strings.Split(contentType, ";")
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("invalid content type")
	}
	
	mediaType = strings.TrimSpace(parts[0])
	params = make(map[string]string)
	
	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if kv := strings.SplitN(part, "=", 2); len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.Trim(strings.TrimSpace(kv[1]), "\"")
			params[key] = value
		}
	}
	
	return mediaType, params, nil
}

func processFilePart(part *multipart.Part, filename string, userID uuid.UUID) (string, error) {
	uploadDir, err := manager.CreateUserTempDir(userID)
	if err != nil {
		return "", fmt.Errorf("could not create upload directory: %w", err)
	}
	
	dstPath := filepath.Join(uploadDir, filepath.Base(filename))
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return "", fmt.Errorf("could not create file on disk: %w", err)
	}
	defer dstFile.Close()
	
	_, err = io.Copy(dstFile, part)
	if err != nil {
		return "", fmt.Errorf("could not save file: %w", err)
	}
	
	return dstPath, nil
}
