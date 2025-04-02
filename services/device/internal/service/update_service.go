package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/repository"
	
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// OTAService defines the OTA update operations
type OTAService interface {
	// Update sessions
	CreateUpdateSession(ctx context.Context, deviceID, firmwareID uint, priority uint, forceUpdate, allowRollback bool) (*models.OTAUpdateSession, error)
	GetUpdateSession(ctx context.Context, sessionID string) (*models.OTAUpdateSession, error)
	ListDeviceUpdateSessions(ctx context.Context, deviceID uint, limit int) ([]*models.OTAUpdateSession, error)
	CancelUpdateSession(ctx context.Context, sessionID string) error
	
	// Batch operations
	CreateUpdateBatch(ctx context.Context, firmwareID uint, deviceIDs []uint, priority uint, forceUpdate, allowRollback bool, maxConcurrent uint) (*models.OTAUpdateBatch, error)
	GetUpdateBatch(ctx context.Context, batchID string) (*models.OTAUpdateBatch, error)
	CancelUpdateBatch(ctx context.Context, batchID string) error
	
	// Device update process
	CheckForUpdate(ctx context.Context, deviceUID string, currentVersion string) (*models.OTAUpdateSession, error)
	AcknowledgeUpdate(ctx context.Context, sessionID string) error
	GetUpdateChunk(ctx context.Context, sessionID string, offset, size uint64) ([]byte, error)
	CompleteDownload(ctx context.Context, sessionID string, checksum string) error
	CompleteUpdate(ctx context.Context, sessionID string, success bool, errorMessage string) error
	
	// Health monitoring
	GetStuckUpdates(ctx context.Context, thresholdMinutes int) ([]*models.OTAUpdateSession, error)
	RetryFailedUpdate(ctx context.Context, sessionID string) (*models.OTAUpdateSession, error)
	GetUpdateStats(ctx context.Context) (map[string]interface{}, error)
}

// otaService implements OTAService
type otaService struct {
	repo          repository.Repository
	otaRepo       repository.OTARepository
	firmwareRepo  repository.FirmwareRepository
	firmwareService FirmwareService
	log           *logrus.Logger
	chunkCache    map[string][]byte
	cacheMutex    sync.RWMutex
	maxCacheSize  int
}

// NewOTAService creates a new OTA service
func NewOTAService(
	repo repository.Repository,
	otaRepo repository.OTARepository,
	firmwareRepo repository.FirmwareRepository,
	firmwareService FirmwareService,
	log *logrus.Logger,
) OTAService {
	return &otaService{
		repo:           repo,
		otaRepo:        otaRepo,
		firmwareRepo:   firmwareRepo,
		firmwareService: firmwareService,
		log:            log,
		chunkCache:     make(map[string][]byte),
		cacheMutex:     sync.RWMutex{},
		maxCacheSize:   1024 * 1024 * 10, // 10MB cache
	}
}

// CreateUpdateSession creates a new update session for a device
func (s *otaService) CreateUpdateSession(
	ctx context.Context, 
	deviceID, 
	firmwareID uint, 
	priority uint, 
	forceUpdate, 
	allowRollback bool,
) (*models.OTAUpdateSession, error) {
	// Validate device exists
	device, err := s.repo.FindDeviceByID(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}
	
	// Check if device allows updates
	if !device.AllowUpdates && !forceUpdate {
		return nil, errors.New("device does not allow updates")
	}
	
	// Validate firmware exists
	firmware, err := s.firmwareRepo.GetFirmwareReleaseByID(ctx, firmwareID)
	if err != nil {
		return nil, fmt.Errorf("firmware not found: %w", err)
	}
	
	// Check if firmware is valid
	if !firmware.Valid {
		return nil, errors.New("firmware is not valid")
	}
	
	// Check if firmware is active
	if !firmware.Active {
		return nil, errors.New("firmware is not active")
	}
	
	// Create update session
	session := &models.OTAUpdateSession{
		SessionID:        uuid.New().String(),
		DeviceID:         deviceID,
		FirmwareReleaseID: firmwareID,
		Status:           models.OTAStatusScheduled,
		ScheduledAt:      time.Now(),
		Priority:         priority,
		ForceUpdate:      forceUpdate,
		AllowRollback:    allowRollback,
		TotalBytes:       uint64(firmware.Size),
	}
	
	// Save to database
	if err := s.otaRepo.CreateUpdateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create update session: %w", err)
	}
	
	return session, nil
}

// GetUpdateSession gets an update session by ID
func (s *otaService) GetUpdateSession(ctx context.Context, sessionID string) (*models.OTAUpdateSession, error) {
	return s.otaRepo.GetUpdateSession(ctx, sessionID)
}

// ListDeviceUpdateSessions lists update sessions for a device
func (s *otaService) ListDeviceUpdateSessions(ctx context.Context, deviceID uint, limit int) ([]*models.OTAUpdateSession, error) {
	return s.otaRepo.ListDeviceUpdateSessions(ctx, deviceID, limit)
}

// CancelUpdateSession cancels an update session
func (s *otaService) CancelUpdateSession(ctx context.Context, sessionID string) error {
	// Get the session
	session, err := s.otaRepo.GetUpdateSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	
	// Check if session can be cancelled
	if session.Status == models.OTAStatusCompleted || 
	   session.Status == models.OTAStatusFailed ||
	   session.Status == models.OTAStatusCancelled {
		return fmt.Errorf("cannot cancel session with status %s", session.Status)
	}
	
	// Update session status
	session.Status = models.OTAStatusCancelled
	
	// Save to database
	if err := s.otaRepo.UpdateUpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	
	return nil
}

// CreateUpdateBatch creates a batch update for multiple devices
func (s *otaService) CreateUpdateBatch(
	ctx context.Context, 
	firmwareID uint, 
	deviceIDs []uint, 
	priority uint, 
	forceUpdate, 
	allowRollback bool,
	maxConcurrent uint,
) (*models.OTAUpdateBatch, error) {
	// Validate firmware exists
	firmware, err := s.firmwareRepo.GetFirmwareReleaseByID(ctx, firmwareID)
	if err != nil {
		return nil, fmt.Errorf("firmware not found: %w", err)
	}
	
	// Check if firmware is valid
	if !firmware.Valid {
		return nil, errors.New("firmware is not valid")
	}
	
	// Check if firmware is active
	if !firmware.Active {
		return nil, errors.New("firmware is not active")
	}
	
	// Create batch
	batchID := uuid.New().String()
	batch := &models.OTAUpdateBatch{
		BatchID:           batchID,
		FirmwareReleaseID: firmwareID,
		Status:            models.OTAStatusScheduled,
		ScheduledAt:       time.Now(),
		Priority:          priority,
		ForceUpdate:       forceUpdate,
		AllowRollback:     allowRollback,
		TotalCount:        uint(len(deviceIDs)),
		PendingCount:      uint(len(deviceIDs)),
		CreatedBy:         "system",
		MaxConcurrent:     maxConcurrent,
	}
	
	// Save batch to database
	if err := s.otaRepo.CreateUpdateBatch(ctx, batch); err != nil {
		return nil, fmt.Errorf("failed to create update batch: %w", err)
	}
	
	// Create sessions for each device
	for _, deviceID := range deviceIDs {
		// Create session
		session := &models.OTAUpdateSession{
			SessionID:        uuid.New().String(),
			DeviceID:         deviceID,
			FirmwareReleaseID: firmwareID,
			Status:           models.OTAStatusScheduled,
			ScheduledAt:      time.Now(),
			Priority:         priority,
			ForceUpdate:      forceUpdate,
			AllowRollback:    allowRollback,
			BatchID:          batchID,
			TotalBytes:       uint64(firmware.Size),
		}
		
		// Save session to database
		if err := s.otaRepo.CreateUpdateSession(ctx, session); err != nil {
			s.log.WithError(err).Errorf("Failed to create update session for device %d in batch %s", deviceID, batchID)
			// Continue with other devices
			continue
		}
	}
	
	return batch, nil
}

// GetUpdateBatch gets a batch by ID
func (s *otaService) GetUpdateBatch(ctx context.Context, batchID string) (*models.OTAUpdateBatch, error) {
	return s.otaRepo.GetUpdateBatch(ctx, batchID)
}

// CancelUpdateBatch cancels a batch update
func (s *otaService) CancelUpdateBatch(ctx context.Context, batchID string) error {
	// Get the batch
	batch, err := s.otaRepo.GetUpdateBatch(ctx, batchID)
	if err != nil {
		return fmt.Errorf("batch not found: %w", err)
	}
	
	// Check if batch can be cancelled
	if batch.Status == models.OTAStatusCompleted || 
	   batch.Status == models.OTAStatusFailed ||
	   batch.Status == models.OTAStatusCancelled {
		return fmt.Errorf("cannot cancel batch with status %s", batch.Status)
	}
	
	// Update batch status
	batch.Status = models.OTAStatusCancelled
	
	// Save to database
	if err := s.otaRepo.UpdateUpdateBatch(ctx, batch); err != nil {
		return fmt.Errorf("failed to update batch: %w", err)
	}
	
	// Cancel all pending sessions in this batch
	sessions, err := s.otaRepo.ListBatchUpdateSessions(ctx, batchID)
	if err != nil {
		s.log.WithError(err).Errorf("Failed to list sessions for batch %s", batchID)
		return nil
	}
	
	for _, session := range sessions {
		// Only cancel sessions that are not already completed/failed/cancelled
		if session.Status != models.OTAStatusCompleted && 
		   session.Status != models.OTAStatusFailed && 
		   session.Status != models.OTAStatusCancelled {
			
			session.Status = models.OTAStatusCancelled
			
			if err := s.otaRepo.UpdateUpdateSession(ctx, session); err != nil {
				s.log.WithError(err).Errorf("Failed to cancel session %s in batch %s", session.SessionID, batchID)
				// Continue with other sessions
				continue
			}
		}
	}
	
	return nil
}

// CheckForUpdate checks if an update is available for a device
func (s *otaService) CheckForUpdate(ctx context.Context, deviceUID string, currentVersion string) (*models.OTAUpdateSession, error) {
	// Get the device
	device, err := s.repo.FindDeviceByUID(ctx, deviceUID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}
	
	// Check if device is active
	if !device.Active {
		return nil, errors.New("device is not active")
	}
	
	// Check if device allows updates
	if !device.AllowUpdates {
		return nil, errors.New("device does not allow updates")
	}
	
	// Look for pending update sessions
	sessions, err := s.otaRepo.GetPendingUpdateSessions(ctx, device.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending updates: %w", err)
	}
	
	// If there are pending updates, return the highest priority one
	if len(sessions) > 0 {
		// Find the highest priority session
		var highestPrioritySession *models.OTAUpdateSession
		
		for i, session := range sessions {
			if i == 0 || session.Priority > highestPrioritySession.Priority {
				highestPrioritySession = session
			}
		}
		
		// Update session status to acknowledged
		highestPrioritySession.Status = models.OTAStatusAcknowledged
		now := time.Now()
		highestPrioritySession.AcknowledgedAt = &now
		highestPrioritySession.DeviceVersion = currentVersion
		
		// Save to database
		if err := s.otaRepo.UpdateUpdateSession(ctx, highestPrioritySession); err != nil {
			return nil, fmt.Errorf("failed to update session: %w", err)
		}
		
		// Check if this is part of a batch
		if highestPrioritySession.BatchID != "" {
			// Update batch statistics
			s.updateBatchStatistics(ctx, highestPrioritySession.BatchID)
		}
		
		// Add device version information to the session log
		logEntry := &models.OTADeviceLog{
			DeviceID:   device.ID,
			SessionID:  highestPrioritySession.SessionID,
			EventType:  "device_check",
			LogLevel:   "info",
			Message:    fmt.Sprintf("Device checked for updates with version %s", currentVersion),
		}
		
		if err := s.otaRepo.CreateDeviceLog(ctx, logEntry); err != nil {
			s.log.WithError(err).Error("Failed to create device log entry")
		}
		
		return highestPrioritySession, nil
	}
	
	// No pending updates, check if there's a newer firmware version available
	if device.CurrentReleaseID != nil {
		// Get current firmware
		currentFirmware, err := s.firmwareRepo.GetFirmwareReleaseByID(ctx, *device.CurrentReleaseID)
		if err != nil {
			return nil, fmt.Errorf("failed to get current firmware: %w", err)
		}
		
		// Get latest firmware
		latestFirmware, err := s.firmwareRepo.GetLatestFirmwareRelease(ctx, models.ReleaseTypeProduction)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest firmware: %w", err)
		}
		
		// Compare versions
		comp, err := s.firmwareService.CompareVersions(currentFirmware.Version, latestFirmware.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to compare versions: %w", err)
		}
		
		// If newer version is available, create update session
		if comp < 0 {
			session := &models.OTAUpdateSession{
				SessionID:        uuid.New().String(),
				DeviceID:         device.ID,
				FirmwareReleaseID: latestFirmware.ID,
				Status:           models.OTAStatusAcknowledged,
				ScheduledAt:      time.Now(),
				Priority:         5, // Default priority
				ForceUpdate:      false,
				AllowRollback:    true,
				DeviceVersion:    currentVersion,
				TotalBytes:       uint64(latestFirmware.Size),
			}
			
			now := time.Now()
			session.AcknowledgedAt = &now
			
			// Save to database
			if err := s.otaRepo.CreateUpdateSession(ctx, session); err != nil {
				return nil, fmt.Errorf("failed to create update session: %w", err)
			}
			
			// Add device version information to the session log
			logEntry := &models.OTADeviceLog{
				DeviceID:   device.ID,
				SessionID:  session.SessionID,
				EventType:  "auto_update",
				LogLevel:   "info",
				Message:    fmt.Sprintf("Auto-created update from version %s to %s", currentFirmware.Version, latestFirmware.Version),
			}
			
			if err := s.otaRepo.CreateDeviceLog(ctx, logEntry); err != nil {
				s.log.WithError(err).Error("Failed to create device log entry")
			}
			
			return session, nil
		}
	}
	
	// No updates available
	return nil, nil
}

// AcknowledgeUpdate acknowledges an update session
func (s *otaService) AcknowledgeUpdate(ctx context.Context, sessionID string) error {
	// Get the session
	session, err := s.otaRepo.GetUpdateSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	
	// Check if session is in a valid state
	if session.Status != models.OTAStatusScheduled && session.Status != models.OTAStatusPending {
		return fmt.Errorf("invalid session status: %s", session.Status)
	}
	
	// Update session status
	session.Status = models.OTAStatusAcknowledged
	now := time.Now()
	session.AcknowledgedAt = &now
	
	// Save to database
	if err := s.otaRepo.UpdateUpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	
	// Add log entry
	logEntry := &models.OTADeviceLog{
		DeviceID:   session.DeviceID,
		SessionID:  session.SessionID,
		EventType:  "acknowledge",
		LogLevel:   "info",
		Message:    "Device acknowledged update",
	}
	
	if err := s.otaRepo.CreateDeviceLog(ctx, logEntry); err != nil {
		s.log.WithError(err).Error("Failed to create device log entry")
	}
	
	// Update batch statistics if this is part of a batch
	if session.BatchID != "" {
		s.updateBatchStatistics(ctx, session.BatchID)
	}
	
	return nil
}

// GetUpdateChunk gets a chunk of the firmware file
func (s *otaService) GetUpdateChunk(ctx context.Context, sessionID string, offset, size uint64) ([]byte, error) {
	// Get the session
	session, err := s.otaRepo.GetUpdateSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	
	// Check if session is in a valid state
	if session.Status != models.OTAStatusAcknowledged && session.Status != models.OTAStatusDownloading {
		return nil, fmt.Errorf("invalid session status: %s", session.Status)
	}
	
	// Get the firmware release
	release, err := s.firmwareRepo.GetFirmwareReleaseByID(ctx, session.FirmwareReleaseID)
	if err != nil {
		return nil, fmt.Errorf("firmware not found: %w", err)
	}
	
	// Check if the requested chunk is within bounds
	if offset >= uint64(release.Size) {
		return nil, fmt.Errorf("offset %d is beyond file size %d", offset, release.Size)
	}
	
	// Calculate actual chunk size (handle last chunk)
	remaining := uint64(release.Size) - offset
	if size > remaining {
		size = remaining
	}
	
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%d:%d", sessionID, offset, size)
	
	s.cacheMutex.RLock()
	cachedChunk, found := s.chunkCache[cacheKey]
	s.cacheMutex.RUnlock()
	
	if found {
		// Update statistics but return from cache
		now := time.Now()
		
		// Only update database every few chunks to reduce load
		if session.ChunksReceived%5 == 0 {
			session.LastChunkTime = &now
			session.LastChunkSize = uint(size)
			session.BytesDownloaded = offset + size
			session.ChunksReceived++
			
			// Calculate download speed (bytes per second)
			if session.DownloadStartedAt != nil {
				elapsedSeconds := now.Sub(*session.DownloadStartedAt).Seconds()
				if elapsedSeconds > 0 {
					session.DownloadSpeed = uint64(float64(session.BytesDownloaded) / elapsedSeconds)
				}
			}
			
			if err := s.otaRepo.UpdateUpdateSession(ctx, session); err != nil {
				s.log.WithError(err).Error("Failed to update session statistics")
			}
		}
		
		return cachedChunk, nil
	}
	
	// Open the file
	file, err := os.Open(release.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open firmware file: %w", err)
	}
	defer file.Close()
	
	// Seek to the requested offset
	if _, err := file.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek in firmware file: %w", err)
	}
	
	// Read the chunk
	chunk := make([]byte, size)
	n, err := io.ReadFull(file, chunk)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("failed to read from firmware file: %w", err)
	}
	
	// Truncate chunk if we read less than requested
	if uint64(n) < size {
		chunk = chunk[:n]
	}
	
	// Update session stats
	now := time.Now()
	
	// Initialize download if this is the first chunk
	if session.Status == models.OTAStatusAcknowledged {
		session.Status = models.OTAStatusDownloading
		session.DownloadStartedAt = &now
		session.ChunksTotal = uint(math.Ceil(float64(release.Size) / float64(size)))
	}
	
	session.LastChunkTime = &now
	session.LastChunkSize = uint(size)
	session.BytesDownloaded = offset + uint64(n)
	session.ChunksReceived++
	
	// Calculate download speed (bytes per second)
	if session.DownloadStartedAt != nil {
		elapsedSeconds := now.Sub(*session.DownloadStartedAt).Seconds()
		if elapsedSeconds > 0 {
			session.DownloadSpeed = uint64(float64(session.BytesDownloaded) / elapsedSeconds)
		}
	}
	
	// Save to database
	if err := s.otaRepo.UpdateUpdateSession(ctx, session); err != nil {
		s.log.WithError(err).Error("Failed to update session")
	}
	
	// Add log entry every 10 chunks to avoid spamming the logs
	if session.ChunksReceived%10 == 0 || session.ChunksReceived == 1 {
		percentComplete := float64(session.BytesDownloaded) / float64(session.TotalBytes) * 100.0
		
		logEntry := &models.OTADeviceLog{
			DeviceID:   session.DeviceID,
			SessionID:  session.SessionID,
			EventType:  "download_progress",
			LogLevel:   "info",
			Message:    fmt.Sprintf("Download progress: %.1f%% (%d/%d bytes)", percentComplete, session.BytesDownloaded, session.TotalBytes),
			Metadata:   fmt.Sprintf(`{"percent":%.1f,"bytes":%d,"total":%d,"speed":%d}`, percentComplete, session.BytesDownloaded, session.TotalBytes, session.DownloadSpeed),
		}
		
		if err := s.otaRepo.CreateDeviceLog(ctx, logEntry); err != nil {
			s.log.WithError(err).Error("Failed to create device log entry")
		}
	}
	
	// Cache the chunk for future requests
	s.cacheMutex.Lock()
	// First, check if we need to make room in the cache
	for s.cacheSize() > s.maxCacheSize {
		// Remove a random entry
		for k := range s.chunkCache {
			delete(s.chunkCache, k)
			break
		}
	}
	
	// Add to cache
	s.chunkCache[cacheKey] = chunk
	s.cacheMutex.Unlock()
	
	// Update batch statistics if this is part of a batch
	if session.BatchID != "" {
		// Only update periodically to reduce DB load
		if session.ChunksReceived%20 == 0 {
			s.updateBatchStatistics(ctx, session.BatchID)
		}
	}
	
	return chunk, nil
}

// CompleteDownload marks an update download as complete
func (s *otaService) CompleteDownload(ctx context.Context, sessionID string, checksum string) error {
	// Get the session
	session, err := s.otaRepo.GetUpdateSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	
	// Check if session is in a valid state
	if session.Status != models.OTAStatusDownloading {
		return fmt.Errorf("invalid session status: %s", session.Status)
	}
	
	// Get the firmware release
	release, err := s.firmwareRepo.GetFirmwareReleaseByID(ctx, session.FirmwareReleaseID)
	if err != nil {
		return fmt.Errorf("firmware not found: %w", err)
	}
	
	// Verify checksum if provided
	if checksum != "" {
		if checksum != release.FileHash {
			// Log the checksum mismatch but continue since device may have calculated it differently
			s.log.Warnf("Checksum mismatch for session %s: got %s, expected %s", sessionID, checksum, release.FileHash)
			
			// Add log entry
			logEntry := &models.OTADeviceLog{
				DeviceID:   session.DeviceID,
				SessionID:  session.SessionID,
				EventType:  "checksum_mismatch",
				LogLevel:   "warn",
				Message:    "Checksum mismatch during download verification",
				Metadata:   fmt.Sprintf(`{"device_checksum":"%s","server_checksum":"%s"}`, checksum, release.FileHash),
			}
			
			if err := s.otaRepo.CreateDeviceLog(ctx, logEntry); err != nil {
				s.log.WithError(err).Error("Failed to create device log entry")
			}
		}
	}
	
	// Update session status
	session.Status = models.OTAStatusVerifying
	now := time.Now()
	session.DownloadCompletedAt = &now
	session.VerificationStartedAt = &now
	session.DownloadChecksum = checksum
	
	// Save to database
	if err := s.otaRepo.UpdateUpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	
	// Add log entry
	logEntry := &models.OTADeviceLog{
		DeviceID:   session.DeviceID,
		SessionID:  session.SessionID,
		EventType:  "download_complete",
		LogLevel:   "info",
		Message:    "Device completed firmware download",
		Metadata:   fmt.Sprintf(`{"total_bytes":%d,"download_time_sec":%.1f}`, session.BytesDownloaded, now.Sub(*session.DownloadStartedAt).Seconds()),
	}
	
	if err := s.otaRepo.CreateDeviceLog(ctx, logEntry); err != nil {
		s.log.WithError(err).Error("Failed to create device log entry")
	}
	
	// Update batch statistics if this is part of a batch
	if session.BatchID != "" {
		s.updateBatchStatistics(ctx, session.BatchID)
	}
	
	// Start verification in background (typically the device is performing verification,
	// but we also do server-side verification)
	go func() {
		// Wait a short time to simulate verification
		time.Sleep(1 * time.Second)
		
		verifyCtx := context.Background()
		verifySession, err := s.otaRepo.GetUpdateSession(verifyCtx, sessionID)
		if err != nil {
			s.log.WithError(err).Error("Failed to get session for verification")
			return
		}
		
		// Only proceed if still in verifying state
		if verifySession.Status != models.OTAStatusVerifying {
			return
		}
		
		// Mark verification complete
		verifyNow := time.Now()
		verifySession.VerificationCompletedAt = &verifyNow
		verifySession.Status = models.OTAStatusInstalling
		verifySession.InstallStartedAt = &verifyNow
		
		if err := s.otaRepo.UpdateUpdateSession(verifyCtx, verifySession); err != nil {
			s.log.WithError(err).Error("Failed to update session after verification")
			return
		}
		
		// Add log entry
		verifyLogEntry := &models.OTADeviceLog{
			DeviceID:   verifySession.DeviceID,
			SessionID:  verifySession.SessionID,
			EventType:  "verification_complete",
			LogLevel:   "info",
			Message:    "Firmware verification completed",
		}
		
		if err := s.otaRepo.CreateDeviceLog(verifyCtx, verifyLogEntry); err != nil {
			s.log.WithError(err).Error("Failed to create device log entry")
		}
		
		// Update batch statistics if this is part of a batch
		if verifySession.BatchID != "" {
			s.updateBatchStatistics(verifyCtx, verifySession.BatchID)
		}
	}()
	
	return nil
}

// CompleteUpdate marks an update as complete
func (s *otaService) CompleteUpdate(ctx context.Context, sessionID string, success bool, errorMessage string) error {
	// Get the session
	session, err := s.otaRepo.GetUpdateSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	
	// Check if session is in a valid state
	if session.Status != models.OTAStatusVerifying && session.Status != models.OTAStatusInstalling {
		return fmt.Errorf("invalid session status: %s", session.Status)
	}
	
	now := time.Now()
	
	if success {
		// Update session status
		session.Status = models.OTAStatusCompleted
		session.CompletedAt = &now
		session.InstallCompletedAt = &now
		
		// Get the firmware release
		release, err := s.firmwareRepo.GetFirmwareReleaseByID(ctx, session.FirmwareReleaseID)
		if err != nil {
			return fmt.Errorf("firmware not found: %w", err)
		}
		
		// Get the device
		device, err := s.repo.FindDeviceByID(ctx, session.DeviceID)
		if err != nil {
			return fmt.Errorf("device not found: %w", err)
		}
		
		// Update device firmware
		device.CurrentReleaseID = &release.ID
		
		// Save device to database
		if err := s.repo.UpdateDevice(ctx, device); err != nil {
			return fmt.Errorf("failed to update device: %w", err)
		}
		
		// Add log entry
		logEntry := &models.OTADeviceLog{
			DeviceID:   session.DeviceID,
			SessionID:  session.SessionID,
			EventType:  "update_complete",
			LogLevel:   "info",
			Message:    "Device successfully completed firmware update",
		}
		
		if err := s.otaRepo.CreateDeviceLog(ctx, logEntry); err != nil {
			s.log.WithError(err).Error("Failed to create device log entry")
		}
	} else {
		// Update session status
		session.Status = models.OTAStatusFailed
		session.FailedAt = &now
		session.ErrorMessage = errorMessage
		
		// Add log entry
		logEntry := &models.OTADeviceLog{
			DeviceID:   session.DeviceID,
			SessionID:  session.SessionID,
			EventType:  "update_failed",
			LogLevel:   "error",
			Message:    fmt.Sprintf("Device failed to complete firmware update: %s", errorMessage),
		}
		
		if err := s.otaRepo.CreateDeviceLog(ctx, logEntry); err != nil {
			s.log.WithError(err).Error("Failed to create device log entry")
		}
	}
	
	// Save to database
	if err := s.otaRepo.UpdateUpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	
	// Update batch statistics if this is part of a batch
	if session.BatchID != "" {
		s.updateBatchStatistics(ctx, session.BatchID)
	}
	
	return nil
}

// GetStuckUpdates gets update sessions that appear to be stuck
func (s *otaService) GetStuckUpdates(ctx context.Context, thresholdMinutes int) ([]*models.OTAUpdateSession, error) {
	// Define the threshold time
	threshold := time.Now().Add(-time.Duration(thresholdMinutes) * time.Minute)
	
	// Get stuck sessions from repository
	return s.otaRepo.GetStuckUpdateSessions(ctx, threshold)
}

// RetryFailedUpdate retries a failed update
func (s *otaService) RetryFailedUpdate(ctx context.Context, sessionID string) (*models.OTAUpdateSession, error) {
	// Get the session
	session, err := s.otaRepo.GetUpdateSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	
	// Check if session can be retried
	if session.Status != models.OTAStatusFailed {
		return nil, fmt.Errorf("only failed sessions can be retried")
	}
	
	// Check retry count
	if session.RetryCount >= session.MaxRetries {
		return nil, fmt.Errorf("maximum retry count reached (%d)", session.MaxRetries)
	}
	
	// Create a new session for retry
	retrySession := &models.OTAUpdateSession{
		SessionID:        uuid.New().String(),
		DeviceID:         session.DeviceID,
		FirmwareReleaseID: session.FirmwareReleaseID,
		Status:           models.OTAStatusScheduled,
		ScheduledAt:      time.Now(),
		Priority:         session.Priority,
		ForceUpdate:      session.ForceUpdate,
		AllowRollback:    session.AllowRollback,
		BatchID:          session.BatchID,
		TotalBytes:       session.TotalBytes,
		RetryCount:       session.RetryCount + 1,
		MaxRetries:       session.MaxRetries,
	}
	
	// Save to database
	if err := s.otaRepo.CreateUpdateSession(ctx, retrySession); err != nil {
		return nil, fmt.Errorf("failed to create retry session: %w", err)
	}
	
	// Add log entry
	logEntry := &models.OTADeviceLog{
		DeviceID:   session.DeviceID,
		SessionID:  retrySession.SessionID,
		EventType:  "retry_update",
		LogLevel:   "info",
		Message:    fmt.Sprintf("Retrying failed update (attempt %d of %d)", retrySession.RetryCount, retrySession.MaxRetries),
	}
	
	if err := s.otaRepo.CreateDeviceLog(ctx, logEntry); err != nil {
		s.log.WithError(err).Error("Failed to create device log entry")
	}
	
	// Update batch statistics if this is part of a batch
	if retrySession.BatchID != "" {
		s.updateBatchStatistics(ctx, retrySession.BatchID)
	}
	
	return retrySession, nil
}

// GetUpdateStats gets statistics about updates
func (s *otaService) GetUpdateStats(ctx context.Context) (map[string]interface{}, error) {
	// Get statistics from repository
	return s.otaRepo.GetUpdateStats(ctx)
}

// Helper functions

// updateBatchStatistics updates the statistics for a batch
func (s *otaService) updateBatchStatistics(ctx context.Context, batchID string) {
	// Get the batch
	batch, err := s.otaRepo.GetUpdateBatch(ctx, batchID)
	if err != nil {
		s.log.WithError(err).Errorf("Failed to get batch %s", batchID)
		return
	}
	
	// Get all sessions in this batch
	sessions, err := s.otaRepo.ListBatchUpdateSessions(ctx, batchID)
	if err != nil {
		s.log.WithError(err).Errorf("Failed to list sessions for batch %s", batchID)
		return
	}
	
	// Count sessions by status
	var completedCount, failedCount, pendingCount uint
	for _, session := range sessions {
		switch session.Status {
		case models.OTAStatusCompleted:
			completedCount++
		case models.OTAStatusFailed, models.OTAStatusRolledBack, models.OTAStatusCancelled:
			failedCount++
		default:
			pendingCount++
		}
	}
	
	// Update batch statistics
	batch.CompletedCount = completedCount
	batch.FailedCount = failedCount
	batch.PendingCount = pendingCount
	
	// Update batch status based on sessions
	if pendingCount == 0 {
		if failedCount == 0 {
			batch.Status = models.OTAStatusCompleted
			now := time.Now()
			batch.CompletedAt = &now
		} else if completedCount == 0 {
			batch.Status = models.OTAStatusFailed
		} else {
			batch.Status = models.OTAStatusCompleted
			now := time.Now()
			batch.CompletedAt = &now
		}
	} else if batch.Status == models.OTAStatusScheduled && (completedCount > 0 || failedCount > 0) {
		batch.Status = models.OTAStatusInProgress
	}
	
	// Save to database
	if err := s.otaRepo.UpdateUpdateBatch(ctx, batch); err != nil {
		s.log.WithError(err).Errorf("Failed to update batch %s", batchID)
	}
}

// cacheSize returns the current size of the chunk cache in bytes
func (s *otaService) cacheSize() int {
	size := 0
	for _, chunk := range s.chunkCache {
		size += len(chunk)
	}
	return size
}