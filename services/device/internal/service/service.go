package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	
	"example.com/backstage/services/device/internal/cache"
	"example.com/backstage/services/device/internal/messaging"
	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/repository"
	
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// Service defines the business logic operations
type Service interface {
	// Device operations
	RegisterDevice(ctx context.Context, device *models.Device) error
	GetDevice(ctx context.Context, id uint) (*models.Device, error)
	GetDeviceByUID(ctx context.Context, uid string) (*models.Device, error)
	ListDevices(ctx context.Context, orgID uint) ([]*models.Device, error)
	UpdateDeviceStatus(ctx context.Context, id uint, active bool) error
	AssignFirmwareToDevice(ctx context.Context, deviceID, releaseID uint) error
	
	// DeviceMessage operations
	ProcessDeviceMessage(ctx context.Context, message *models.DeviceMessage) error
	GetDeviceMessages(ctx context.Context, deviceID uint, limit int) ([]*models.DeviceMessage, error)
	
	// Organization operations
	CreateOrganization(ctx context.Context, org *models.Organization) error
	UpdateOrganization(ctx context.Context, org *models.Organization) error
	GetOrganization(ctx context.Context, id uint) (*models.Organization, error)
	ListOrganizations(ctx context.Context) ([]*models.Organization, error)
	
	// FirmwareRelease operations
	CreateFirmwareRelease(ctx context.Context, release *models.FirmwareRelease) error
	GetFirmwareRelease(ctx context.Context, id uint) (*models.FirmwareRelease, error)
	ListFirmwareReleases(ctx context.Context, releaseType models.ReleaseType) ([]*models.FirmwareRelease, error)
	ActivateFirmwareRelease(ctx context.Context, id uint) error
}

// service is an implementation of the Service interface
type service struct {
	repo            repository.Repository
	cache           cache.RedisClient
	messagingClient messaging.ServiceBusClient
	log             *logrus.Logger
}

// NewService creates a new service instance
func NewService(
	repo repository.Repository, 
	cache cache.RedisClient,
	messagingClient messaging.ServiceBusClient,
	log *logrus.Logger,
) Service {
	return &service{
		repo:            repo,
		cache:           cache,
		messagingClient: messagingClient,
		log:             log,
	}
}

// Device operations implementation

func (s *service) RegisterDevice(ctx context.Context, device *models.Device) error {
	// Generate UUID if not provided
	if device.UID == "" {
		device.UID = uuid.New().String()
	}
	
	// Set default values
	device.Active = true
	device.AllowUpdates = true
	
	if err := s.repo.CreateDevice(ctx, device); err != nil {
		return fmt.Errorf("failed to create device: %w", err)
	}
	
	// Cache the device info
	deviceJSON, err := json.Marshal(device)
	if err == nil {
		s.cache.Set(ctx, fmt.Sprintf("device:%s", device.UID), string(deviceJSON), 24*time.Hour)
	}
	
	return nil
}

func (s *service) GetDevice(ctx context.Context, id uint) (*models.Device, error) {
	// Try to get from database
	device, err := s.repo.FindDeviceByID(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// Update cache
	deviceJSON, err := json.Marshal(device)
	if err == nil {
		s.cache.Set(ctx, fmt.Sprintf("device:%s", device.UID), string(deviceJSON), 24*time.Hour)
	}
	
	return device, nil
}

func (s *service) GetDeviceByUID(ctx context.Context, uid string) (*models.Device, error) {
	// Try to get from cache first
	cacheKey := fmt.Sprintf("device:%s", uid)
	cachedData, err := s.cache.Get(ctx, cacheKey)
	if err == nil {
		var device models.Device
		if err := json.Unmarshal([]byte(cachedData), &device); err == nil {
			return &device, nil
		}
	}
	
	// Fallback to database
	device, err := s.repo.FindDeviceByUID(ctx, uid)
	if err != nil {
		return nil, err
	}
	
	// Update cache
	deviceJSON, err := json.Marshal(device)
	if err == nil {
		s.cache.Set(ctx, cacheKey, string(deviceJSON), 24*time.Hour)
	}
	
	return device, nil
}

func (s *service) ListDevices(ctx context.Context, orgID uint) ([]*models.Device, error) {
	return s.repo.ListDevices(ctx, orgID)
}

func (s *service) UpdateDeviceStatus(ctx context.Context, id uint, active bool) error {
	device, err := s.repo.FindDeviceByID(ctx, id)
	if err != nil {
		return err
	}
	
	device.Active = active
	
	if err := s.repo.UpdateDevice(ctx, device); err != nil {
		return err
	}
	
	// Update cache
	deviceJSON, err := json.Marshal(device)
	if err == nil {
		s.cache.Set(ctx, fmt.Sprintf("device:%s", device.UID), string(deviceJSON), 24*time.Hour)
	}
	
	return nil
}

func (s *service) AssignFirmwareToDevice(ctx context.Context, deviceID, releaseID uint) error {
	device, err := s.repo.FindDeviceByID(ctx, deviceID)
	if err != nil {
		return err
	}
	
	release, err := s.repo.FindFirmwareReleaseByID(ctx, releaseID)
	if err != nil {
		return err
	}
	
	// Check if release is active
	if !release.Active {
		return fmt.Errorf("cannot assign inactive firmware release")
	}
	
	// Assign the release to the device
	device.CurrentReleaseID = &release.ID
	
	if err := s.repo.UpdateDevice(ctx, device); err != nil {
		return err
	}
	
	// Update cache
	deviceJSON, err := json.Marshal(device)
	if err == nil {
		s.cache.Set(ctx, fmt.Sprintf("device:%s", device.UID), string(deviceJSON), 24*time.Hour)
	}
	
	return nil
}

// DeviceMessage operations implementation

func (s *service) ProcessDeviceMessage(ctx context.Context, message *models.DeviceMessage) error {
	// Ensure the message has a UUID
	if message.UUID == "" {
		message.UUID = uuid.New().String()
	}
	
	// Find the device by MCU
	device, err := s.repo.FindDeviceByUID(ctx, message.DeviceMCU)
	if err != nil {
		// Create an error message
		message.Error = true
		message.ErrorMessage = fmt.Sprintf("Device not found: %s", message.DeviceMCU)
	} else {
		message.DeviceID = device.ID
		message.Device = device
	}
	
	// Save the message
	if err := s.repo.SaveDeviceMessage(ctx, message); err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}
	
	// // If device was found, publish the message to the message queue
	// if !message.Error {
	// 	// Use device UID as the session ID
	// 	sessionID := message.DeviceMCU
		
	// 	// Try to publish, but don't fail the entire operation if it doesn't work
	// 	if err := s.messagingClient.SendMessage(ctx, message, sessionID); err != nil {
	// 		s.log.WithError(err).Error("Failed to publish message")
	// 		// We continue processing even if message publishing fails
	// 		// This prevents the API from returning a 500 error to the client
	// 	} else {
	// 		// Mark as published only if successful
	// 		now := time.Now()
	// 		message.Published = true
	// 		message.PublishedAt = &now
			
	// 		if err := s.repo.MarkMessageAsPublished(ctx, message.UUID); err != nil {
	// 			s.log.WithError(err).Error("Failed to mark message as published")
	// 		}
	// 	}
	// }
	
	return nil
}

func (s *service) GetDeviceMessages(ctx context.Context, deviceID uint, limit int) ([]*models.DeviceMessage, error) {
	return s.repo.ListDeviceMessages(ctx, deviceID, limit)
}

// Organization operations implementation

func (s *service) CreateOrganization(ctx context.Context, org *models.Organization) error {
	// Set default values
	org.Active = true
	
	return s.repo.CreateOrganization(ctx, org)
}

func (s *service) UpdateOrganization(ctx context.Context, org *models.Organization) error {
	return s.repo.UpdateOrganization(ctx, org)
}

func (s *service) GetOrganization(ctx context.Context, id uint) (*models.Organization, error) {
	return s.repo.FindOrganizationByID(ctx, id)
}

func (s *service) ListOrganizations(ctx context.Context) ([]*models.Organization, error) {
	return s.repo.ListOrganizations(ctx)
}

// FirmwareRelease operations implementation

func (s *service) CreateFirmwareRelease(ctx context.Context, release *models.FirmwareRelease) error {
	// Set default values if not provided
	if release.ReleaseType == "" {
		release.ReleaseType = models.ReleaseTypeDevelopment
	}
	
	// Validate the release
	if release.FilePath == "" {
		return fmt.Errorf("file path is required")
	}
	
	if release.Version == "" {
		return fmt.Errorf("version is required")
	}
	
	// Create the release
	return s.repo.CreateFirmwareRelease(ctx, release)
}

func (s *service) GetFirmwareRelease(ctx context.Context, id uint) (*models.FirmwareRelease, error) {
	return s.repo.FindFirmwareReleaseByID(ctx, id)
}

func (s *service) ListFirmwareReleases(ctx context.Context, releaseType models.ReleaseType) ([]*models.FirmwareRelease, error) {
	return s.repo.ListFirmwareReleases(ctx, releaseType)
}

func (s *service) ActivateFirmwareRelease(ctx context.Context, id uint) error {
	release, err := s.repo.FindFirmwareReleaseByID(ctx, id)
	if err != nil {
		return err
	}
	
	// Check if the release can be activated
	if !release.Valid {
		return fmt.Errorf("cannot activate invalid release")
	}
	
	if release.IsTest && !release.TestPassed {
		return fmt.Errorf("cannot activate test release that has not passed testing")
	}
	
	// Activate the release
	release.Active = true
	
	return s.repo.UpdateFirmwareRelease(ctx, release)
}
