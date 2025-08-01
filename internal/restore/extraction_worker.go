package restore

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/logging"
	"gorm.io/gorm"
)

type ExtractionWorker struct {
	db            *gorm.DB
	dataDir       string
	mu            sync.RWMutex
	running       bool
	quit          chan struct{}
	emptyPollCount int
}

// Global worker instance for on-demand management
var globalExtractionWorker *ExtractionWorker
var globalExtractionWorkerMu sync.Mutex

func NewExtractionWorker(db *gorm.DB) *ExtractionWorker {
	dataDir := config.Get("DATA_DIR", "")
	if dataDir == "" {
		dataDir = "/data"
	}
	
	return &ExtractionWorker{
		db:      db,
		dataDir: dataDir,
		quit:    make(chan struct{}),
	}
}

func (w *ExtractionWorker) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	go w.run()
}

func (w *ExtractionWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if !w.running {
		return
	}
	
	w.running = false
	close(w.quit)
}

func (w *ExtractionWorker) run() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.quit:
			return
		case <-ticker.C:
			w.processPendingJobs()
		}
	}
}

func (w *ExtractionWorker) processPendingJobs() {
	var jobs []database.RestoreExtractionJob
	if err := w.db.Where("status = ?", "pending").Order("created_at ASC").Find(&jobs).Error; err != nil {
		return
	}

	if len(jobs) == 0 {
		// No jobs found, increment empty poll counter
		w.mu.Lock()
		w.emptyPollCount++
		emptyPolls := w.emptyPollCount
		w.mu.Unlock()
		
		// Auto-shutdown after 15 empty polls (30 seconds with 2s interval)
		if emptyPolls >= 15 {
			logging.Logf("[INFO] Extraction worker shutting down after %d empty polls", emptyPolls)
			w.Stop()
			return
		}
		return
	}

	// Reset empty poll counter when jobs are found
	w.mu.Lock()
	w.emptyPollCount = 0
	w.mu.Unlock()

	for _, job := range jobs {
		w.processJob(job)
	}
}

func (w *ExtractionWorker) processJob(job database.RestoreExtractionJob) {
	now := time.Now()
	job.Status = "extracting"
	job.StartedAt = &now
	job.Progress = 0
	job.StatusMessage = "Starting extraction..."
	
	if err := w.db.Save(&job).Error; err != nil {
		logging.Logf("[ERROR] Failed to update extraction job status: %v", err)
		return
	}
	
	// Check for timeout (if job has been running for more than 30 minutes)
	if job.StartedAt != nil && time.Since(*job.StartedAt) > 30*time.Minute {
		w.failJob(job, "Extraction timed out after 30 minutes")
		return
	}

	// Get the associated RestoreUpload to find the file path
	var upload database.RestoreUpload
	if err := w.db.First(&upload, job.RestoreUploadID).Error; err != nil {
		w.failJob(job, fmt.Sprintf("Failed to find restore upload: %v", err))
		return
	}

	// Create temporary extraction directory
	extractDir := filepath.Join(w.dataDir, "temp", "extractions", job.ID.String())
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		w.failJob(job, fmt.Sprintf("Failed to create extraction directory: %v", err))
		return
	}

	// Update progress
	job.Progress = 10
	job.StatusMessage = "Extracting archive..."
	w.db.Save(&job)

	// Extract the tar.gz file
	if err := w.extractTarGz(upload.FilePath, extractDir, &job); err != nil {
		// Clean up on failure
		os.RemoveAll(extractDir)
		w.failJob(job, fmt.Sprintf("Extraction failed: %v", err))
		return
	}

	// Mark as completed
	completedAt := time.Now()
	job.Status = "completed"
	job.Progress = 100
	job.StatusMessage = "Extraction completed"
	job.ExtractedPath = extractDir
	job.CompletedAt = &completedAt
	
	if err := w.db.Save(&job).Error; err != nil {
		logging.Logf("[ERROR] Failed to mark extraction job as completed: %v", err)
	}

	logging.Logf("[INFO] Extraction job %s completed successfully, extracted to: %s", job.ID, extractDir)
}

func (w *ExtractionWorker) extractTarGz(archivePath, destDir string, job *database.RestoreExtractionJob) error {
	// Channel for async progress updates (buffered to avoid blocking)
	type progressUpdate struct {
		progress int
		message  string
	}
	progressChan := make(chan progressUpdate, 10)
	
	// Goroutine to handle DB updates asynchronously
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		lastSavedProgress := -1
		
		for update := range progressChan {
			// Calculate scaled progress
			scaledProgress := 10 + (update.progress * 80 / 100)
			
			// Only save to DB if progress changed by at least 5% or at completion
			if scaledProgress - lastSavedProgress >= 5 || update.progress == 100 || update.progress == 0 {
				job.Progress = scaledProgress
				job.StatusMessage = update.message
				
				if err := w.db.Save(job).Error; err != nil {
					logging.Logf("[WARNING] Failed to update extraction progress: %v", err)
				}
				
				lastSavedProgress = scaledProgress
			}
		}
	}()
	
	// Extract with non-blocking progress updates
	err := ExtractTarGzWithProgress(archivePath, destDir, func(progress int, message string) {
		// Send progress update without blocking extraction
		select {
		case progressChan <- progressUpdate{progress, message}:
			// Progress update sent
		default:
			// Channel full, skip this update to avoid blocking extraction
		}
	})
	
	// Close channel and wait for final updates to complete
	close(progressChan)
	wg.Wait()
	
	return err
}

func (w *ExtractionWorker) failJob(job database.RestoreExtractionJob, errorMsg string) {
	now := time.Now()
	job.Status = "failed"
	job.ErrorMessage = errorMsg
	job.CompletedAt = &now
	job.Progress = 0
	job.StatusMessage = "Extraction failed"
	
	if err := w.db.Save(&job).Error; err != nil {
		logging.Logf("[ERROR] Failed to mark extraction job as failed: %v", err)
	}
	
	logging.Logf("[ERROR] Extraction job %s failed: %s", job.ID, errorMsg)
}

// CreateExtractionJob creates a new extraction job for a restore upload
func CreateExtractionJob(db *gorm.DB, adminUserID, restoreUploadID uuid.UUID) (*database.RestoreExtractionJob, error) {
	// Check if an extraction job already exists for this upload
	var existingJob database.RestoreExtractionJob
	err := db.Where("restore_upload_id = ?", restoreUploadID).First(&existingJob).Error
	if err == nil {
		// Job already exists, return it
		return &existingJob, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("error checking for existing extraction job: %w", err)
	}

	// Create new extraction job
	job := database.RestoreExtractionJob{
		AdminUserID:     adminUserID,
		RestoreUploadID: restoreUploadID,
		Status:          "pending",
		Progress:        0,
		StatusMessage:   "Queued for extraction",
	}

	if err := db.Create(&job).Error; err != nil {
		return nil, fmt.Errorf("failed to create extraction job: %w", err)
	}

	logging.Logf("[INFO] Created extraction job %s for upload %s", job.ID, restoreUploadID)
	return &job, nil
}

// CreateCompletedExtractionJob creates a completed extraction job with existing extracted files
func CreateCompletedExtractionJob(db *gorm.DB, adminUserID, restoreUploadID uuid.UUID, extractionPath string) (*database.RestoreExtractionJob, error) {
	// Check if an extraction job already exists for this upload
	var existingJob database.RestoreExtractionJob
	err := db.Where("restore_upload_id = ?", restoreUploadID).First(&existingJob).Error
	if err == nil {
		// Job already exists, return it
		return &existingJob, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("error checking for existing extraction job: %w", err)
	}

	// Get data directory for moving files to proper extraction job structure
	dataDir := config.Get("DATA_DIR", "")
	if dataDir == "" {
		dataDir = "/data"
	}

	// Create extraction job with proper UUID-based directory name
	job := database.RestoreExtractionJob{
		AdminUserID:     adminUserID,
		RestoreUploadID: restoreUploadID,
		Status:          "completed",
		Progress:        100,
		StatusMessage:   "Extraction completed (reused from analysis)",
	}

	// Create the job to get the UUID
	if err := db.Create(&job).Error; err != nil {
		return nil, fmt.Errorf("failed to create extraction job: %w", err)
	}

	// Move extracted files to proper extraction job directory
	finalExtractionDir := filepath.Join(dataDir, "temp", "extractions", job.ID.String())
	if err := os.Rename(extractionPath, finalExtractionDir); err != nil {
		// If move fails, cleanup the job and return error
		db.Delete(&job)
		return nil, fmt.Errorf("failed to move extracted files from %s to %s: %w", extractionPath, finalExtractionDir, err)
	}

	// Update job with final path and timestamps
	now := time.Now()
	job.ExtractedPath = finalExtractionDir
	job.StartedAt = &now
	job.CompletedAt = &now

	if err := db.Save(&job).Error; err != nil {
		// If saving fails, cleanup and return error
		os.RemoveAll(finalExtractionDir)
		db.Delete(&job)
		return nil, fmt.Errorf("failed to update extraction job: %w", err)
	}

	logging.Logf("[INFO] Created completed extraction job %s for upload %s, reused files from: %s", job.ID, restoreUploadID, finalExtractionDir)
	return &job, nil
}

// GetExtractionJob retrieves an extraction job by ID
func GetExtractionJob(db *gorm.DB, jobID uuid.UUID, adminUserID uuid.UUID) (*database.RestoreExtractionJob, error) {
	var job database.RestoreExtractionJob
	err := db.Where("id = ? AND admin_user_id = ?", jobID, adminUserID).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// GetExtractionJobByUpload retrieves an extraction job by restore upload ID
func GetExtractionJobByUpload(db *gorm.DB, uploadID uuid.UUID, adminUserID uuid.UUID) (*database.RestoreExtractionJob, error) {
	var job database.RestoreExtractionJob
	err := db.Where("restore_upload_id = ? AND admin_user_id = ?", uploadID, adminUserID).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// CleanupExtractionJob removes the extraction files and job record
func CleanupExtractionJob(db *gorm.DB, jobID uuid.UUID, adminUserID uuid.UUID) error {
	var job database.RestoreExtractionJob
	if err := db.Where("id = ? AND admin_user_id = ?", jobID, adminUserID).First(&job).Error; err != nil {
		return err
	}

	// Clean up extracted files if they exist
	if job.ExtractedPath != "" {
		if err := os.RemoveAll(job.ExtractedPath); err != nil {
			logging.Logf("[WARNING] Failed to remove extraction directory %s: %v", job.ExtractedPath, err)
		} else {
			logging.Logf("[INFO] Cleaned up extraction directory: %s", job.ExtractedPath)
		}
	}

	// Delete the job record
	return db.Delete(&job).Error
}

// CleanupExpiredExtractions removes old extraction jobs and their files
func CleanupExpiredExtractions(db *gorm.DB) error {
	// Find extraction jobs older than 24 hours
	cutoff := time.Now().Add(-24 * time.Hour)
	var oldJobs []database.RestoreExtractionJob
	if err := db.Where("created_at < ?", cutoff).Find(&oldJobs).Error; err != nil {
		return err
	}

	for _, job := range oldJobs {
		// Clean up extraction files
		if job.ExtractedPath != "" {
			if err := os.RemoveAll(job.ExtractedPath); err != nil {
				logging.Logf("[WARNING] Failed to remove old extraction directory %s: %v", job.ExtractedPath, err)
			}
		}
		
		// Delete job record
		if err := db.Delete(&job).Error; err != nil {
			logging.Logf("[WARNING] Failed to delete old extraction job %s: %v", job.ID, err)
		}
	}

	if len(oldJobs) > 0 {
		logging.Logf("[INFO] Cleaned up %d expired extraction jobs", len(oldJobs))
	}

	return nil
}

// ExtractTarGzWithProgress extracts a tar.gz archive with progress reporting
func ExtractTarGzWithProgress(archivePath, destDir string, progressCallback func(int, string)) error {
	// Get file size for progress calculation
	stat, err := os.Stat(archivePath)
	if err != nil {
		return fmt.Errorf("failed to stat archive: %w", err)
	}
	totalSize := stat.Size()
	
	// Open archive file
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	// Create progress reader
	progressReader := &ProgressReader{
		Reader:     file,
		TotalSize:  totalSize,
		OnProgress: progressCallback,
	}

	// Create gzip reader
	gzReader, err := gzip.NewReader(progressReader)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	fileCount := 0
	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		fileCount++
		if fileCount%10 == 0 {
			progressCallback(-1, fmt.Sprintf("Extracted %d files...", fileCount))
		}

		// Calculate destination path
		destPath := filepath.Join(destDir, header.Name)

		// Ensure the destination is within destDir (security check)
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(destPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", destPath, err)
			}

		case tar.TypeReg:
			// Create file
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", destPath, err)
			}

			outFile, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", destPath, err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write file %s: %w", destPath, err)
			}

			outFile.Close()

			// Set file permissions
			if err := os.Chmod(destPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to set permissions for %s: %w", destPath, err)
			}
		}
	}

	progressCallback(100, fmt.Sprintf("Extraction completed - %d files extracted", fileCount))
	return nil
}

// ProgressReader wraps an io.Reader to provide progress updates
type ProgressReader struct {
	Reader     io.Reader
	TotalSize  int64
	ReadBytes  int64
	OnProgress func(int, string)
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.ReadBytes += int64(n)
	
	if pr.OnProgress != nil && pr.TotalSize > 0 {
		progress := int((pr.ReadBytes * 100) / pr.TotalSize)
		if progress > 100 {
			progress = 100
		}
		pr.OnProgress(progress, fmt.Sprintf("Reading archive... %d%%", progress))
	}
	
	return n, err
}

// EnsureWorkerRunning starts the extraction worker if it's not already running
func EnsureWorkerRunning(db *gorm.DB) {
	globalExtractionWorkerMu.Lock()
	defer globalExtractionWorkerMu.Unlock()
	
	// If worker exists and is running, nothing to do
	if globalExtractionWorker != nil && globalExtractionWorker.IsRunning() {
		return
	}
	
	// Create and start new worker
	globalExtractionWorker = NewExtractionWorker(db)
	globalExtractionWorker.Start()
	logging.Logf("[INFO] Extraction worker started on-demand")
}

// IsRunning returns true if the worker is currently running
func (w *ExtractionWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// GetWorkerStatus returns the current status of the global extraction worker for debugging
func GetWorkerStatus() map[string]interface{} {
	globalExtractionWorkerMu.Lock()
	defer globalExtractionWorkerMu.Unlock()
	
	if globalExtractionWorker == nil {
		return map[string]interface{}{
			"exists": false,
			"running": false,
		}
	}
	
	globalExtractionWorker.mu.RLock()
	status := map[string]interface{}{
		"exists": true,
		"running": globalExtractionWorker.running,
		"empty_poll_count": globalExtractionWorker.emptyPollCount,
	}
	globalExtractionWorker.mu.RUnlock()
	
	return status
}