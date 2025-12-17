package database

import (
	"math/rand"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a user account in the system
type User struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"` // Never return password in JSON
	IsAdmin   bool      `gorm:"default:false" json:"is_admin"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	
	// User-specific settings
	RmapiHost    string `gorm:"column:rmapi_host" json:"rmapi_host,omitempty"`
	DefaultRmdir string `gorm:"column:default_rmdir;default:/" json:"default_rmdir"`
	FolderRefreshPercent int `gorm:"column:folder_refresh_percent;default:0" json:"folder_refresh_percent"`
	CoverpageSetting string `gorm:"column:coverpage_setting" json:"coverpage_setting"`
	ConflictResolution string `gorm:"column:conflict_resolution;default:abort" json:"conflict_resolution"`
	FolderDepthLimit int `gorm:"column:folder_depth_limit;default:0" json:"folder_depth_limit"`
	FolderExclusionList string `gorm:"column:folder_exclusion_list" json:"folder_exclusion_list"`
	PageResolution string `gorm:"column:page_resolution" json:"page_resolution,omitempty"`
	PageDPI float64 `gorm:"column:page_dpi" json:"page_dpi,omitempty"`
	ConversionOutputFormat string `gorm:"column:conversion_output_format;default:epub" json:"conversion_output_format,omitempty"`
	RmapiConfig string `gorm:"column:rmapi_config;type:text" json:"-"` // Never return config in JSON

	// PDF processing settings
	PDFBackgroundRemoval bool `gorm:"column:pdf_background_removal;default:false" json:"pdf_background_removal"`
	
	// Password reset
	ResetToken        string    `gorm:"index" json:"-"`
	ResetTokenExpires time.Time `json:"-"`
	
	
	// OIDC integration
	OidcSubject *string `gorm:"column:oidc_subject;uniqueIndex" json:"oidc_subject,omitempty"`
	
	// Timestamps
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	LastLogin *time.Time `json:"last_login,omitempty"`
	
	// Associations
	APIKeys       []APIKey       `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Sessions      []UserSession  `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	FoldersCache  []FolderCache  `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Documents     []Document     `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

// BeforeCreate sets UUID and randomized folder refresh minute if not already set
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	if u.FolderRefreshPercent == 0 {
		// Assign a random percentage (1-99) for folder refresh scheduling
		u.FolderRefreshPercent = rand.Intn(99) + 1
	}
	return nil
}

// APIKey represents an API key for a user
type APIKey struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Name      string    `gorm:"not null" json:"name"`
	KeyHash   string    `gorm:"not null;index" json:"-"` // Never return actual key
	KeyPrefix string    `gorm:"size:16;not null" json:"key_prefix"` // First 16 chars for display
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	
	// Association
	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (a *APIKey) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// UserSession represents a user's login session
type UserSession struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash string    `gorm:"not null;index" json:"-"`
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"last_used"`
	UserAgent string    `gorm:"type:text" json:"user_agent,omitempty"`
	IPAddress string    `gorm:"size:45" json:"ip_address,omitempty"`
	
	// Association
	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (s *UserSession) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// FolderCache represents cached folder data for a user
type FolderCache struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	FolderPath  string    `gorm:"size:1000;not null" json:"folder_path"`
	FolderData  string    `gorm:"type:text" json:"folder_data"` // JSON data
	LastUpdated time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"last_updated"`
	
	// Association
	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (f *FolderCache) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

// Add unique constraint for user_id + folder_path
func (FolderCache) TableName() string {
	return "user_folders_cache"
}

// Document represents a document uploaded/downloaded by a user
type Document struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	DocumentName string    `gorm:"not null" json:"document_name"`
	LocalPath    string    `gorm:"size:1000" json:"local_path,omitempty"`
	RemotePath   string    `gorm:"size:1000" json:"remote_path,omitempty"`
	DocumentType string    `gorm:"size:50" json:"document_type,omitempty"`
	FileSize     int64     `json:"file_size,omitempty"`
	Status       string    `gorm:"size:50;default:uploaded" json:"status"`
	UploadDate   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"upload_date"`
	
	// Association
	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (d *Document) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

// SystemSetting represents system-wide configuration
type SystemSetting struct {
	Key         string    `gorm:"primaryKey" json:"key"`
	Value       string    `gorm:"type:text" json:"value"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	UpdatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
	UpdatedBy   *uuid.UUID `gorm:"type:uuid" json:"updated_by,omitempty"`
	
	// Association
	UpdatedByUser *User `gorm:"foreignKey:UpdatedBy" json:"-"`
}

// LoginAttempt represents a login attempt for rate limiting
type LoginAttempt struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	IPAddress   string    `gorm:"size:45;not null;index" json:"ip_address"`
	Username    string    `gorm:"index" json:"username,omitempty"`
	Success     bool      `gorm:"default:false" json:"success"`
	AttemptedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;index" json:"attempted_at"`
	UserAgent   string    `gorm:"type:text" json:"user_agent,omitempty"`
}

func (l *LoginAttempt) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

// BackupJob represents a background backup operation
type BackupJob struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	AdminUserID   uuid.UUID `gorm:"type:uuid;not null;index" json:"admin_user_id"`
	Status        string    `gorm:"size:50;not null;default:pending" json:"status"`
	Progress      int       `gorm:"default:0" json:"progress"`
	IncludeFiles  bool      `gorm:"default:true" json:"include_files"`
	IncludeConfigs bool     `gorm:"default:true" json:"include_configs"`
	UserIDs       string    `gorm:"type:text" json:"user_ids,omitempty"`
	FilePath      string    `gorm:"size:1000" json:"file_path,omitempty"`
	Filename      string    `gorm:"size:255" json:"filename,omitempty"`
	FileSize      int64     `json:"file_size,omitempty"`
	ErrorMessage  string    `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	
	// Association
	AdminUser User `gorm:"foreignKey:AdminUserID;constraint:OnDelete:CASCADE" json:"-"`
}

// RestoreUpload represents an uploaded restore file waiting for confirmation
type RestoreUpload struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	AdminUserID uuid.UUID `gorm:"type:uuid;not null;index" json:"admin_user_id"`
	Filename    string    `gorm:"size:255;not null" json:"filename"`
	FilePath    string    `gorm:"size:1000;not null" json:"file_path"`
	FileSize    int64     `json:"file_size"`
	Status      string    `gorm:"size:50;not null;default:uploaded" json:"status"`
	ExpiresAt   time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	
	// Association
	AdminUser User `gorm:"foreignKey:AdminUserID;constraint:OnDelete:CASCADE" json:"-"`
}

func (b *BackupJob) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

func (r *RestoreUpload) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// RestoreExtractionJob represents a background tar extraction operation for restore
type RestoreExtractionJob struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	AdminUserID     uuid.UUID `gorm:"type:uuid;not null;index" json:"admin_user_id"`
	RestoreUploadID uuid.UUID `gorm:"type:uuid;not null;index" json:"restore_upload_id"`
	Status          string    `gorm:"size:50;not null;default:pending" json:"status"`
	Progress        int       `gorm:"default:0" json:"progress"`
	StatusMessage   string    `gorm:"type:text" json:"status_message,omitempty"`
	ErrorMessage    string    `gorm:"type:text" json:"error_message,omitempty"`
	ExtractedPath   string    `gorm:"size:1000" json:"extracted_path,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	
	// Associations
	AdminUser     User          `gorm:"foreignKey:AdminUserID;constraint:OnDelete:CASCADE" json:"-"`
	RestoreUpload RestoreUpload `gorm:"foreignKey:RestoreUploadID;constraint:OnDelete:CASCADE" json:"-"`
}

func (r *RestoreExtractionJob) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}


// GetAllModels returns all models for auto-migration
func GetAllModels() []interface{} {
	return []interface{}{
		&User{},
		&APIKey{},
		&UserSession{},
		&FolderCache{},
		&Document{},
		&SystemSetting{},
		&LoginAttempt{},
		&BackupJob{},
		&RestoreUpload{},
		&RestoreExtractionJob{},
	}
}