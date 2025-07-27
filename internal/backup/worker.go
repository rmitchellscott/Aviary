package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/database"
	"github.com/rmitchellscott/aviary/internal/export"
	"gorm.io/gorm"
)

type Worker struct {
	db      *gorm.DB
	dataDir string
	mu      sync.RWMutex
	running bool
	quit    chan struct{}
}

func NewWorker(db *gorm.DB) *Worker {
	dataDir := config.Get("DATA_DIR", "")
	if dataDir == "" {
		dataDir = "/data"
	}
	
	return &Worker{
		db:      db,
		dataDir: dataDir,
		quit:    make(chan struct{}),
	}
}

func (w *Worker) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	go w.run()
}

func (w *Worker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if !w.running {
		return
	}
	
	w.running = false
	close(w.quit)
}

func (w *Worker) run() {
	ticker := time.NewTicker(5 * time.Second)
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

func (w *Worker) processPendingJobs() {
	var jobs []database.BackupJob
	if err := w.db.Where("status = ?", "pending").Order("created_at ASC").Find(&jobs).Error; err != nil {
		return
	}

	for _, job := range jobs {
		w.processJob(job)
	}
}

func (w *Worker) processJob(job database.BackupJob) {
	now := time.Now()
	job.Status = "running"
	job.StartedAt = &now
	job.Progress = 0
	
	if err := w.db.Save(&job).Error; err != nil {
		return
	}

	backupDir := filepath.Join(w.dataDir, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		w.failJob(job, fmt.Sprintf("Failed to create backup directory: %v", err))
		return
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("aviary_backup_%s.tar.gz", timestamp)
	backupPath := filepath.Join(backupDir, filename)

	exporter := export.NewExporter(w.db, w.dataDir)
	
	var userIDs []uuid.UUID
	if job.UserIDs != "" {
		for _, idStr := range strings.Split(job.UserIDs, ",") {
			if id, err := uuid.Parse(strings.TrimSpace(idStr)); err == nil {
				userIDs = append(userIDs, id)
			}
		}
	}

	exportOptions := export.ExportOptions{
		IncludeDatabase: true,
		IncludeFiles:    job.IncludeFiles,
		IncludeConfigs:  job.IncludeConfigs,
		UserIDs:         userIDs,
	}

	job.Progress = 50
	w.db.Save(&job)

	if err := exporter.Export(backupPath, exportOptions); err != nil {
		w.failJob(job, fmt.Sprintf("Export failed: %v", err))
		return
	}

	stat, err := os.Stat(backupPath)
	if err != nil {
		w.failJob(job, fmt.Sprintf("Failed to get backup file info: %v", err))
		return
	}

	completedAt := time.Now()
	expiresAt := completedAt.Add(24 * time.Hour)
	
	job.Status = "completed"
	job.Progress = 100
	job.FilePath = backupPath
	job.Filename = filename
	job.FileSize = stat.Size()
	job.CompletedAt = &completedAt
	job.ExpiresAt = &expiresAt
	
	w.db.Save(&job)
}

func (w *Worker) failJob(job database.BackupJob, errorMsg string) {
	now := time.Now()
	job.Status = "failed"
	job.ErrorMessage = errorMsg
	job.CompletedAt = &now
	w.db.Save(&job)
}

func CreateBackupJob(db *gorm.DB, adminUserID uuid.UUID, includeFiles, includeConfigs bool, userIDs []uuid.UUID) (*database.BackupJob, error) {
	var userIDsStr string
	if len(userIDs) > 0 {
		var strs []string
		for _, id := range userIDs {
			strs = append(strs, id.String())
		}
		userIDsStr = strings.Join(strs, ",")
	}

	job := database.BackupJob{
		AdminUserID:    adminUserID,
		Status:         "pending",
		IncludeFiles:   includeFiles,
		IncludeConfigs: includeConfigs,
		UserIDs:        userIDsStr,
	}

	if err := db.Create(&job).Error; err != nil {
		return nil, err
	}

	return &job, nil
}

func GetBackupJobs(db *gorm.DB, adminUserID uuid.UUID) ([]database.BackupJob, error) {
	var jobs []database.BackupJob
	err := db.Where("admin_user_id = ?", adminUserID).
		Order("created_at DESC").
		Limit(10).
		Find(&jobs).Error
	return jobs, err
}

func GetBackupJob(db *gorm.DB, jobID uuid.UUID, adminUserID uuid.UUID) (*database.BackupJob, error) {
	var job database.BackupJob
	err := db.Where("id = ? AND admin_user_id = ?", jobID, adminUserID).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func CleanupExpiredBackups(db *gorm.DB) error {
	var expiredJobs []database.BackupJob
	if err := db.Where("expires_at < ? AND status = ?", time.Now(), "completed").Find(&expiredJobs).Error; err != nil {
		return err
	}

	for _, job := range expiredJobs {
		if job.FilePath != "" {
			os.Remove(job.FilePath)
		}
		db.Delete(&job)
	}

	return nil
}

func DeleteBackupJob(db *gorm.DB, jobID uuid.UUID, adminUserID uuid.UUID) error {
	var job database.BackupJob
	if err := db.Where("id = ? AND admin_user_id = ?", jobID, adminUserID).First(&job).Error; err != nil {
		return err
	}

	// Delete the backup file if it exists
	if job.FilePath != "" {
		os.Remove(job.FilePath)
	}

	// Delete the job record
	return db.Delete(&job).Error
}