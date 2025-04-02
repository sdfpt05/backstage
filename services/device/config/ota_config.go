package config

import (
	"path/filepath"
)

// OTAConfig holds configuration for OTA update system
type OTAConfig struct {
	ChunkSize            int    // Size of download chunks in bytes
	MaxConcurrentUpdates int    // Maximum number of concurrent update sessions
	DownloadTimeout      int    // Maximum time in seconds for a download to complete
	MaxRetries           int    // Maximum number of retry attempts
	SessionLifetime      int    // Lifetime of an update session in seconds
	DeltaUpdates         bool   // Whether to use delta updates
	DefaultUpdateType    string // Default update type (full, delta, incremental)
}

// GetRecommendedChunkSize returns the recommended chunk size based on the device type
func (c *OTAConfig) GetRecommendedChunkSize(deviceType string) int {
	// Default to configured chunk size
	chunkSize := c.ChunkSize
	
	// Adjust based on device type
	switch deviceType {
	case "low_memory":
		// Use smaller chunks for devices with limited memory
		return min(chunkSize, 4096) // 4KB max
	case "mobile":
		// Use medium chunks for mobile connections
		return min(chunkSize, 16384) // 16KB max
	case "high_bandwidth":
		// Use larger chunks for high bandwidth connections
		return max(chunkSize, 32768) // 32KB min
	default:
		// Use the configured chunk size
		return chunkSize
	}
}

// BuildUpdateSessionPath builds a path for storing update session files
func (c *OTAConfig) BuildUpdateSessionPath(firmwareConfig *FirmwareConfig, sessionID string) (string, error) {
	storagePath, err := firmwareConfig.GetAbsoluteStoragePath()
	if err != nil {
		return "", err
	}
	
	// Create path for session files
	sessionPath := filepath.Join(storagePath, "sessions", sessionID)
	return sessionPath, nil
}

// CalculateMaxConcurrentUpdatesPerOrg calculates the maximum number of concurrent updates per organization
func (c *OTAConfig) CalculateMaxConcurrentUpdatesPerOrg(orgCount int) int {
	if orgCount <= 0 {
		return c.MaxConcurrentUpdates
	}
	
	// Distribute the limit across organizations, with a minimum per org
	perOrg := max(c.MaxConcurrentUpdates/orgCount, 5)
	return perOrg
}

// IsUpdateTypeSupported checks if an update type is supported
func (c *OTAConfig) IsUpdateTypeSupported(updateType string) bool {
	switch updateType {
	case "full", "delta", "incremental":
		return true
	default:
		return false
	}
}

// ValidateUpdateType validates and normalizes an update type
func (c *OTAConfig) ValidateUpdateType(updateType string) string {
	if updateType == "" {
		return c.DefaultUpdateType
	}
	
	if c.IsUpdateTypeSupported(updateType) {
		return updateType
	}
	
	// Return default if not supported
	return c.DefaultUpdateType
}

// Helper functions to ensure Go 1.23 compatibility
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}