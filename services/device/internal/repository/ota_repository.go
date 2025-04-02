package repository

import (
	"context"
	"fmt"
	"time"

	"example.com/backstage/services/device/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OTA Update Session operations implementation

func (r *repo) CreateOTAUpdateSession(ctx context.Context, session *models.OTAUpdateSession) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}

	// Generate a unique session ID if not provided
	if session.SessionID == "" {
		session.SessionID = uuid.New().String()
	}

	// Set initial status if not provided
	if session.Status == "" {
		session.Status = models.OTAStatusScheduled
	}

	// Set scheduled time if not provided
	if session.ScheduledAt.IsZero() {
		session.ScheduledAt = time.Now()
	}

	return gormDB.Create(session).Error
}

func (r *repo) UpdateOTAUpdateSession(ctx context.Context, session *models.OTAUpdateSession) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}

	return gormDB.Save(session).Error
}

func (r *repo) FindOTAUpdateSessionByID(ctx context.Context, id uint) (*models.OTAUpdateSession, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	var session models.OTAUpdateSession
	if err := gormDB.Preload("Device").Preload("FirmwareRelease").First(&session, id).Error; err != nil {
		return nil, err
	}

	return &session, nil
}

func (r *repo) FindOTAUpdateSessionBySessionID(ctx context.Context, sessionID string) (*models.OTAUpdateSession, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	var session models.OTAUpdateSession
	if err := gormDB.Preload("Device").Preload("FirmwareRelease").
		Where("session_id = ?", sessionID).First(&session).Error; err != nil {
		return nil, err
	}

	return &session, nil
}

func (r *repo) ListOTAUpdateSessionsByDevice(ctx context.Context, deviceID uint, limit int) ([]*models.OTAUpdateSession, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	var sessions []*models.OTAUpdateSession
	query := gormDB.Preload("FirmwareRelease").
		Where("device_id = ?", deviceID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&sessions).Error; err != nil {
		return nil, err
	}

	return sessions, nil
}

func (r *repo) ListOTAUpdateSessionsByStatus(ctx context.Context, status models.OTAUpdateStatus, limit int) ([]*models.OTAUpdateSession, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	var sessions []*models.OTAUpdateSession
	query := gormDB.Preload("Device").Preload("FirmwareRelease").
		Where("status = ?", status).
		Order("created_at ASC") // Process oldest first

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&sessions).Error; err != nil {
		return nil, err
	}

	return sessions, nil
}

func (r *repo) UpdateOTAUpdateSessionStatus(ctx context.Context, sessionID string, status models.OTAUpdateStatus) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}

	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	// Set additional timestamp fields based on status
	switch status {
	case models.OTAStatusAcknowledged:
		now := time.Now()
		updates["acknowledged_at"] = now
	case models.OTAStatusDownloading:
		now := time.Now()
		updates["download_started_at"] = now
	case models.OTAStatusDownloaded:
		now := time.Now()
		updates["download_completed_at"] = now
	case models.OTAStatusVerifying:
		now := time.Now()
		updates["verification_started_at"] = now
	case models.OTAStatusInstalling:
		now := time.Now()
		updates["install_started_at"] = now
	case models.OTAStatusCompleted:
		now := time.Now()
		updates["install_completed_at"] = now
		updates["completed_at"] = now
	case models.OTAStatusFailed:
		now := time.Now()
		updates["failed_at"] = now
	}

	return gormDB.Model(&models.OTAUpdateSession{}).
		Where("session_id = ?", sessionID).
		Updates(updates).Error
}

func (r *repo) UpdateOTAUpdateSessionProgress(ctx context.Context, sessionID string, bytesDownloaded uint64, chunksReceived uint) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}

	now := time.Now()
	
	return gormDB.Model(&models.OTAUpdateSession{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]interface{}{
			"bytes_downloaded": bytesDownloaded,
			"chunks_received":  chunksReceived,
			"last_chunk_time":  now,
			"updated_at":       now,
		}).Error
}

// OTA Update Batch operations implementation

func (r *repo) CreateOTAUpdateBatch(ctx context.Context, batch *models.OTAUpdateBatch) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}

	// Generate a unique batch ID if not provided
	if batch.BatchID == "" {
		batch.BatchID = fmt.Sprintf("batch-%s", uuid.New().String())
	}

	// Set initial status if not provided
	if batch.Status == "" {
		batch.Status = models.OTAStatusScheduled
	}

	// Set scheduled time if not provided
	if batch.ScheduledAt.IsZero() {
		batch.ScheduledAt = time.Now()
	}

	// Run in transaction to create batch and related sessions
	return gormDB.Transaction(func(tx *gorm.DB) error {
		// Save the batch first
		if err := tx.Create(batch).Error; err != nil {
			return err
		}

		// Store the total count
		batch.TotalCount = uint(len(batch.Sessions))
		batch.PendingCount = batch.TotalCount

		// Update the counts
		if err := tx.Save(batch).Error; err != nil {
			return err
		}

		// Process sessions if any
		if len(batch.Sessions) > 0 {
			// Ensure all sessions have the batch ID
			for i := range batch.Sessions {
				batch.Sessions[i].BatchID = batch.BatchID
				
				// Fill in required fields if not provided
				if batch.Sessions[i].SessionID == "" {
					batch.Sessions[i].SessionID = uuid.New().String()
				}
				
				if batch.Sessions[i].Status == "" {
					batch.Sessions[i].Status = models.OTAStatusScheduled
				}
				
				if batch.Sessions[i].ScheduledAt.IsZero() {
					batch.Sessions[i].ScheduledAt = batch.ScheduledAt
				}
				
				batch.Sessions[i].Priority = batch.Priority
				batch.Sessions[i].ForceUpdate = batch.ForceUpdate
				batch.Sessions[i].AllowRollback = batch.AllowRollback
				batch.Sessions[i].FirmwareReleaseID = batch.FirmwareReleaseID
				batch.Sessions[i].UpdateType = batch.UpdateType
			}
			
			// Save all sessions in batches to avoid overwhelming the database
			batchSize := 100
			for i := 0; i < len(batch.Sessions); i += batchSize {
				end := i + batchSize
				if end > len(batch.Sessions) {
					end = len(batch.Sessions)
				}
				
				if err := tx.Create(batch.Sessions[i:end]).Error; err != nil {
					return fmt.Errorf("failed to create update sessions batch %d-%d: %w", i, end, err)
				}
			}
		}

		return nil
	})
}

func (r *repo) UpdateOTAUpdateBatch(ctx context.Context, batch *models.OTAUpdateBatch) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}

	// Update the batch excluding sessions (handled separately if needed)
	return gormDB.Save(batch).Error
}

func (r *repo) FindOTAUpdateBatchByID(ctx context.Context, id uint) (*models.OTAUpdateBatch, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	var batch models.OTAUpdateBatch
	if err := gormDB.Preload("FirmwareRelease").First(&batch, id).Error; err != nil {
		return nil, err
	}

	return &batch, nil
}

func (r *repo) FindOTAUpdateBatchByBatchID(ctx context.Context, batchID string) (*models.OTAUpdateBatch, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	var batch models.OTAUpdateBatch
	if err := gormDB.Preload("FirmwareRelease").
		Where("batch_id = ?", batchID).First(&batch).Error; err != nil {
		return nil, err
	}

	// Get the sessions for this batch
	var sessions []*models.OTAUpdateSession
	if err := gormDB.Preload("Device").Preload("FirmwareRelease").
		Where("batch_id = ?", batchID).Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("failed to get batch sessions: %w", err)
	}

	batch.Sessions = sessions
	return &batch, nil
}

func (r *repo) ListOTAUpdateBatchesByStatus(ctx context.Context, status models.OTAUpdateStatus, limit int) ([]*models.OTAUpdateBatch, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	var batches []*models.OTAUpdateBatch
	query := gormDB.Preload("FirmwareRelease").
		Where("status = ?", status).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&batches).Error; err != nil {
		return nil, err
	}

	return batches, nil
}

// OTA Device Log operations implementation

func (r *repo) LogOTAEvent(ctx context.Context, log *models.OTADeviceLog) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}

	return gormDB.Create(log).Error
}

func (r *repo) GetOTALogsBySession(ctx context.Context, sessionID string, limit int) ([]*models.OTADeviceLog, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	var logs []*models.OTADeviceLog
	query := gormDB.Where("session_id = ?", sessionID).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}

	return logs, nil
}

func (r *repo) GetOTALogsByDevice(ctx context.Context, deviceID uint, limit int) ([]*models.OTADeviceLog, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	var logs []*models.OTADeviceLog
	query := gormDB.Where("device_id = ?", deviceID).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}

	return logs, nil
}

// Health monitoring operations

func (r *repo) FindStaleOTAUpdateSessions(ctx context.Context, threshold time.Duration) ([]*models.OTAUpdateSession, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	thresholdTime := time.Now().Add(-threshold)
	
	var staleSessions []*models.OTAUpdateSession
	
	// Find sessions that have been stuck in an intermediate state for too long
	if err := gormDB.Preload("Device").Preload("FirmwareRelease").
		Where(`(status = ? AND download_started_at < ?) OR 
			  (status = ? AND acknowledged_at < ?) OR 
			  (status = ? AND verification_started_at < ?) OR
			  (status = ? AND install_started_at < ?)`,
			models.OTAStatusDownloading, thresholdTime,
			models.OTAStatusAcknowledged, thresholdTime,
			models.OTAStatusVerifying, thresholdTime,
			models.OTAStatusInstalling, thresholdTime).
		Find(&staleSessions).Error; err != nil {
		return nil, err
	}
	
	return staleSessions, nil
}

func (r *repo) CancelOTAUpdateSession(ctx context.Context, sessionID string, reason string) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}

	// Update the session status and add error message
	now := time.Now()
	return gormDB.Model(&models.OTAUpdateSession{}).
		Where("session_id = ? AND status NOT IN (?, ?, ?)",
			sessionID,
			models.OTAStatusCompleted,
			models.OTAStatusFailed,
			models.OTAStatusCancelled).
		Updates(map[string]interface{}{
			"status":        models.OTAStatusCancelled,
			"error_message": reason,
			"updated_at":    now,
		}).Error
}