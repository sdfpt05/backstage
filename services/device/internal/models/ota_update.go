package models

import (
	"time"
)

// OTAUpdateStatus represents the status of an OTA update process
type OTAUpdateStatus string

const (
	// OTAStatusScheduled represents an update that is scheduled but not yet started
	OTAStatusScheduled OTAUpdateStatus = "scheduled"
	// OTAStatusPending represents an update that is ready to start
	OTAStatusPending OTAUpdateStatus = "pending"
	// OTAStatusAcknowledged represents an update that has been acknowledged by the device
	OTAStatusAcknowledged OTAUpdateStatus = "acknowledged"
	// OTAStatusDownloading represents an update that is currently being downloaded
	OTAStatusDownloading OTAUpdateStatus = "downloading"
	// OTAStatusDownloaded represents an update that has been downloaded completely
	OTAStatusDownloaded OTAUpdateStatus = "downloaded"
	// OTAStatusVerifying represents an update that is being verified
	OTAStatusVerifying OTAUpdateStatus = "verifying"
	// OTAStatusInstalling represents an update that is being installed
	OTAStatusInstalling OTAUpdateStatus = "installing"
	// OTAStatusCompleted represents an update that has been successfully completed
	OTAStatusCompleted OTAUpdateStatus = "completed"
	// OTAStatusFailed represents an update that has failed
	OTAStatusFailed OTAUpdateStatus = "failed"
	// OTAStatusRolledBack represents an update that was rolled back
	OTAStatusRolledBack OTAUpdateStatus = "rolled_back"
	// OTAStatusCancelled represents an update that was cancelled
	OTAStatusCancelled OTAUpdateStatus = "cancelled"
)

// OTAUpdateSession represents a firmware update session
type OTAUpdateSession struct {
	Model
	SessionID          string           `json:"session_id" gorm:"uniqueIndex;Column:session_id"`
	Device             *Device          `json:"device" gorm:"foreignKey:DeviceID"`
	DeviceID           uint             `json:"device_id" gorm:"Column:device_id"`
	FirmwareRelease    *FirmwareRelease `json:"firmware_release" gorm:"foreignKey:FirmwareReleaseID"`
	FirmwareReleaseID  uint             `json:"firmware_release_id" gorm:"Column:firmware_release_id"`
	Status             OTAUpdateStatus  `json:"status" gorm:"Column:status"`
	ScheduledAt        time.Time        `json:"scheduled_at" gorm:"Column:scheduled_at"`
	AcknowledgedAt     *time.Time       `json:"acknowledged_at" gorm:"Column:acknowledged_at"`
	DownloadStartedAt  *time.Time       `json:"download_started_at" gorm:"Column:download_started_at"`
	DownloadCompletedAt *time.Time      `json:"download_completed_at" gorm:"Column:download_completed_at"`
	VerificationStartedAt *time.Time    `json:"verification_started_at" gorm:"Column:verification_started_at"`
	VerificationCompletedAt *time.Time  `json:"verification_completed_at" gorm:"Column:verification_completed_at"`
	InstallStartedAt   *time.Time       `json:"install_started_at" gorm:"Column:install_started_at"`
	InstallCompletedAt *time.Time       `json:"install_completed_at" gorm:"Column:install_completed_at"`
	CompletedAt        *time.Time       `json:"completed_at" gorm:"Column:completed_at"`
	FailedAt           *time.Time       `json:"failed_at" gorm:"Column:failed_at"`
	ErrorMessage       string           `json:"error_message" gorm:"Column:error_message;type:text"`
	RetryCount         uint             `json:"retry_count" gorm:"Column:retry_count"`
	MaxRetries         uint             `json:"max_retries" gorm:"Column:max_retries;default:3"`
	BytesDownloaded    uint64           `json:"bytes_downloaded" gorm:"Column:bytes_downloaded"`
	TotalBytes         uint64           `json:"total_bytes" gorm:"Column:total_bytes"`
	DownloadSpeed      uint64           `json:"download_speed" gorm:"Column:download_speed"`
	LastChunkTime      *time.Time       `json:"last_chunk_time" gorm:"Column:last_chunk_time"`
	LastChunkSize      uint             `json:"last_chunk_size" gorm:"Column:last_chunk_size"`
	ChunksTotal        uint             `json:"chunks_total" gorm:"Column:chunks_total"`
	ChunksReceived     uint             `json:"chunks_received" gorm:"Column:chunks_received"`
	BatchID            string           `json:"batch_id" gorm:"Column:batch_id"`
	Priority           uint             `json:"priority" gorm:"Column:priority;default:5"`
	ForceUpdate        bool             `json:"force_update" gorm:"Column:force_update;default:false"`
	AllowRollback      bool             `json:"allow_rollback" gorm:"Column:allow_rollback;default:true"`
	DeviceVersion      string           `json:"device_version" gorm:"Column:device_version"`
	DownloadChecksum   string           `json:"download_checksum" gorm:"Column:download_checksum"`
	IsRollback         bool             `json:"is_rollback" gorm:"Column:is_rollback"`
	RollbackFromSession *uint           `json:"rollback_from_session" gorm:"Column:rollback_from_session"`
	PreviousVersion    string           `json:"previous_version" gorm:"Column:previous_version"`
	UpdateType         string           `json:"update_type" gorm:"Column:update_type;default:'full'"`
}

// OTAUpdateBatch represents a batch of updates for multiple devices
type OTAUpdateBatch struct {
	Model
	BatchID            string             `json:"batch_id" gorm:"uniqueIndex;Column:batch_id"`
	FirmwareRelease    *FirmwareRelease   `json:"firmware_release" gorm:"foreignKey:FirmwareReleaseID"`
	FirmwareReleaseID  uint               `json:"firmware_release_id" gorm:"Column:firmware_release_id"`
	Sessions           []OTAUpdateSession `json:"sessions" gorm:"foreignKey:BatchID;references:BatchID"`
	Status             OTAUpdateStatus    `json:"status" gorm:"Column:status"`
	ScheduledAt        time.Time          `json:"scheduled_at" gorm:"Column:scheduled_at"`
	Priority           uint               `json:"priority" gorm:"Column:priority;default:5"`
	ForceUpdate        bool               `json:"force_update" gorm:"Column:force_update;default:false"`
	AllowRollback      bool               `json:"allow_rollback" gorm:"Column:allow_rollback;default:true"`
	CompletedCount     uint               `json:"completed_count" gorm:"Column:completed_count"`
	FailedCount        uint               `json:"failed_count" gorm:"Column:failed_count"`
	PendingCount       uint               `json:"pending_count" gorm:"Column:pending_count"`
	TotalCount         uint               `json:"total_count" gorm:"Column:total_count"`
	CreatedBy          string             `json:"created_by" gorm:"Column:created_by"`
	CompletedAt        *time.Time         `json:"completed_at" gorm:"Column:completed_at"`
	Notes              string             `json:"notes" gorm:"Column:notes;type:text"`
	OrganizationID     *uint              `json:"organization_id" gorm:"Column:organization_id"`
	MaxConcurrent      uint               `json:"max_concurrent" gorm:"Column:max_concurrent;default:100"`
	UpdateType         string             `json:"update_type" gorm:"Column:update_type;default:'full'"`
}

// OTADeviceLog represents a log entry for OTA events on a device
type OTADeviceLog struct {
	Model
	Device             *Device            `json:"device" gorm:"foreignKey:DeviceID"`
	DeviceID           uint               `json:"device_id" gorm:"Column:device_id"`
	OTAUpdateSession   *OTAUpdateSession  `json:"ota_update_session" gorm:"foreignKey:SessionID;references:SessionID"`
	SessionID          string             `json:"session_id" gorm:"Column:session_id"`
	EventType          string             `json:"event_type" gorm:"Column:event_type"`
	LogLevel           string             `json:"log_level" gorm:"Column:log_level;default:'info'"`
	Message            string             `json:"message" gorm:"Column:message;type:text"`
	Metadata           string             `json:"metadata" gorm:"Column:metadata;type:text"`
	ErrorCode          *int               `json:"error_code" gorm:"Column:error_code"`
}