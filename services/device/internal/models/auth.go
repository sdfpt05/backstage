// internal/models/api_key.go
package models

import (
	"time"
)

// AuthorizationLevel represents the level of access for an API key
type AuthorizationLevel int

const (
	// NoAuthLevel represents public access with no authentication
	NoAuthLevel AuthorizationLevel = 0
	// ViewerAuthLevel represents read-only access
	ViewerAuthLevel AuthorizationLevel = 1
	// WriterAuthLevel represents read-write access
	WriterAuthLevel AuthorizationLevel = 2
	// SudoAuthLevel represents administrative access
	SudoAuthLevel AuthorizationLevel = 3
	// RegisteredDeviceAuthLevel represents authentication for registered devices
	RegisteredDeviceAuthLevel AuthorizationLevel = 5
)

// APIKey represents an API token with associated access level
type APIKey struct {
	Model
	Key               string            `json:"key" gorm:"uniqueIndex;Column:key"`
	Name              string            `json:"name" gorm:"Column:name"`
	AuthorizationLevel AuthorizationLevel `json:"authorization_level" gorm:"Column:authorization_level"`
	ExpiresAt         *time.Time        `json:"expires_at" gorm:"Column:expires_at"`
	LastUsedAt        *time.Time        `json:"last_used_at" gorm:"Column:last_used_at"`
}