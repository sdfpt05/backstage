package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"example.com/backstage/services/device/config"
	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/service"
	"example.com/backstage/services/device/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// FirmwareUploadHandler handles firmware upload operations
type FirmwareUploadHandler struct {
	service service.Service
	log     *logrus.Logger
	config  *config.FirmwareConfig
}

// NewFirmwareUploadHandler creates a new FirmwareUploadHandler instance
func NewFirmwareUploadHandler(svc service.Service, log *logrus.Logger, cfg *config.FirmwareConfig) *FirmwareUploadHandler {
	return &FirmwareUploadHandler{
		service: svc,
		log:     log,
		config:  cfg,
	}
}

// UploadFirmware handles firmware binary upload with form data
func (h *FirmwareUploadHandler) UploadFirmware(c *gin.Context) {
	// Parse multipart form
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32 MB limit
		h.log.WithError(err).Warn("Failed to parse multipart form")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to parse form data",
		})
		return
	}

	// Get form values
	version := c.PostForm("version")
	releaseType := c.PostForm("type")
	notes := c.PostForm("notes")
	isTestStr := c.PostForm("is_test")
	testDeviceIDStr := c.PostForm("test_device_id")

	// Validate required fields
	if version == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Version is required",
		})
		return
	}

	// Basic validation for version format
	versionPattern := `^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9\.]+)?(\+[a-zA-Z0-9\.]+)?$`
	versionRegex := regexp.MustCompile(versionPattern)
	if !versionRegex.MatchString(version) {
		h.log.WithField("version", version).Warn("Invalid version format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid version format. Must follow semantic versioning (e.g., 1.2.3, 1.2.3-beta, 1.2.3+build123)",
		})
		return
	}

	// Parse semantic version for later use
	semVer, err := utils.ParseSemanticVersion(version)
	if err != nil {
		h.log.WithError(err).Warn("Failed to parse semantic version")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Failed to parse version: %v", err),
		})
		return
	}

	// Validate release type
	if releaseType == "" {
		releaseType = string(models.ReleaseTypeDevelopment) // Default
	} else if !isValidReleaseType(releaseType) {
		h.log.WithField("type", releaseType).Warn("Invalid release type")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid release type. Must be one of: development, qa, test, production",
		})
		return
	}

	// Get file from form
	file, header, err := c.Request.FormFile("firmware")
	if err != nil {
		h.log.WithError(err).Warn("Failed to get firmware file")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to get firmware file",
		})
		return
	}
	defer file.Close()

	// Create storage directory if it doesn't exist
	firmwareDir := h.config.StoragePath
	if err := os.MkdirAll(firmwareDir, 0755); err != nil {
		h.log.WithError(err).Error("Failed to create firmware directory")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create storage directory",
		})
		return
	}

	// Generate filename based on version and timestamp
	timestamp := time.Now().UTC().Format("20060102150405")
	filename := fmt.Sprintf("firmware_v%s_%s_%s.bin", version, releaseType, timestamp)
	filepath := filepath.Join(firmwareDir, filename)

	// Create destination file
	dst, err := os.Create(filepath)
	if err != nil {
		h.log.WithFields(logrus.Fields{
			"filepath": filepath,
			"error":    err.Error(),
		}).Error("Failed to create file")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save firmware file",
		})
		return
	}
	defer func() {
		err := dst.Close()
		if err != nil {
			h.log.WithError(err).Warn("Failed to close destination file")
		}
	}()

	// Calculate hash while copying
	hasher := sha256.New()
	writer := io.MultiWriter(dst, hasher)

	bytesWritten, err := io.Copy(writer, file)
	if err != nil {
		h.log.WithFields(logrus.Fields{
			"filepath": filepath,
			"error":    err.Error(),
		}).Error("Failed to copy file")

		// Attempt to remove the partially written file
		if removeErr := os.Remove(filepath); removeErr != nil {
			h.log.WithError(removeErr).Warn("Failed to remove partial file after error")
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save firmware file",
		})
		return
	}

	// Verify the expected size matches what was written
	if uint64(bytesWritten) != uint64(header.Size) {
		h.log.WithFields(logrus.Fields{
			"expected": header.Size,
			"actual":   bytesWritten,
		}).Warn("File size mismatch")
	}

	// Get file hash
	fileHash := hex.EncodeToString(hasher.Sum(nil))

	// Parse test device ID if provided
	var testDeviceID *uint
	if testDeviceIDStr != "" {
		id, err := ParseUint(testDeviceIDStr)
		if err == nil {
			uintID := uint(id)
			testDeviceID = &uintID
		}
	}

	// Determine if this is a test release
	isTest := isTestStr == "true" || isTestStr == "1"

	// Create firmware release record
	release := &models.FirmwareRelease{
		FilePath:     filepath,
		ReleaseType:  models.ReleaseType(releaseType),
		Version:      version,
		Size:         uint(header.Size),
		Valid:        true, // Will be validated later
		IsTest:       isTest,
		TestDeviceID: testDeviceID,
		FileHash:     fileHash,
	}

	// Create extended firmware release
	extendedRelease := &models.FirmwareReleaseExtended{
		FirmwareRelease:   *release,
		MajorVersion:      semVer.Major,
		MinorVersion:      semVer.Minor,
		PatchVersion:      semVer.Patch,
		PreReleaseVersion: semVer.PreRelease,
		BuildMetadata:     semVer.Build,
		ReleaseNotes:      notes,
	}

	// Sign the firmware if enabled
	if h.config.VerifySignatures {
		if _, err := h.config.GetPrivateKeyPath(); err != nil {
			h.log.WithError(err).Error("Failed to get private key path")
			extendedRelease.Error = fmt.Sprintf("Signing failed: %v", err)
		} else {
			keyPair, err := utils.LoadSigningKeyPair(h.config.SigningAlgorithm, h.config.KeysPath)
			if err != nil {
				h.log.WithError(err).Error("Failed to load signing key")
				extendedRelease.Error = fmt.Sprintf("Signing failed: %v", err)
			} else {
				signature, err := utils.SignFirmware(keyPair, filepath)
				if err != nil {
					h.log.WithError(err).Error("Failed to sign firmware")
					// Continue without signature, but mark as not signed
					extendedRelease.Error = fmt.Sprintf("Signing failed: %v", err)
				} else {
					extendedRelease.Signature = signature
					extendedRelease.SignatureAlgorithm = h.config.SigningAlgorithm
					now := time.Now()
					extendedRelease.SignedAt = &now
					extendedRelease.SignedBy = "system"
				}
			}
		}
	}

	// Save to database
	if err := h.service.CreateFirmwareRelease(c, &extendedRelease.FirmwareRelease); err != nil {
		h.log.WithError(err).Error("Failed to create firmware release")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create firmware release record",
		})
		return
	}

	// Start validation process asynchronously
	go func() {
		if _, err := h.service.ValidateFirmware(context.Background(), extendedRelease.ID); err != nil {
			h.log.WithError(err).Error("Failed to verify firmware")
		}
	}()

	c.JSON(http.StatusCreated, gin.H{
		"status":         "success",
		"id":             extendedRelease.ID,
		"version":        version,
		"file_path":      filepath,
		"size":           header.Size,
		"hash":           fileHash,
		"release_type":   releaseType,
		"is_test":        isTest,
		"test_device_id": testDeviceID,
	})
}

// ParseUint is a helper function to parse uint values from strings
func ParseUint(s string) (uint64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	return strconv.ParseUint(s, 10, 64)
}

// Function to check if a release type is valid
func isValidReleaseType(releaseType string) bool {
	validTypes := []string{
		string(models.ReleaseTypeDevelopment),
		string(models.ReleaseTypeTest),
		string(models.ReleaseTypeProduction),
	}

	for _, t := range validTypes {
		if releaseType == t {
			return true
		}
	}
	return false
}
