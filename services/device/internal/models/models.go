package models

import (
	"time"
	
	"gorm.io/gorm"
)

// Model is the base model with common fields
type Model struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// ReleaseType is an enum for firmware release types
type ReleaseType string

const (
	// ReleaseTypeProduction represents a production release
	ReleaseTypeProduction ReleaseType = "production"
	// ReleaseTypeDevelopment represents a development release
	ReleaseTypeDevelopment ReleaseType = "development"
	// ReleaseTypeTest represents a test release
	ReleaseTypeTest ReleaseType = "test"
)

// Organization model
type Organization struct {
	Model
	Name    string `json:"name" gorm:"Column:name"`
	URI     string `json:"uri" gorm:"Column:uri"`
	Active  bool   `json:"active" gorm:"Column:active"`
	Persist bool   `json:"persist" gorm:"Column:persist"`
}

// FirmwareRelease model
type FirmwareRelease struct {
	Model
	FilePath      string           `json:"file_path" gorm:"Column:file_path"`
	ReleaseType   ReleaseType      `json:"type" gorm:"Column:release_type"`
	Version       string           `json:"version" gorm:"Column:version"`
	Error         string           `json:"error" gorm:"Column:error"`
	Active        bool             `json:"active" gorm:"Column:active"`
	Size          uint             `json:"size"`
	Valid         bool             `json:"valid" gorm:"Column:valid"`
	IsTest        bool             `json:"is_test" gorm:"Column:is_test"`
	TestRelease   *FirmwareRelease `json:"test_release" gorm:"foreignKey:TestReleaseID"`
	TestReleaseID *uint            `json:"-" gorm:"Column:test_release_id"`
	TestDevice    *Device          `json:"-" gorm:"foreignkey:TestDeviceID"`
	TestDeviceID  *uint            `json:"test_device_id" gorm:"Column:test_device_id"`
	TestPassed    bool             `json:"test_passed" gorm:"Column:test_passed"`
	FileHash      string           `json:"file_hash" gorm:"-"`
}

// Device model
type Device struct {
	Model
	UID             string           `json:"uid" gorm:"Column:uuid"`
	Serial          *string          `json:"serial" gorm:"Column:serial"`
	Organization    *Organization    `json:"organization" gorm:"foreignKey:OrganizationID"`
	OrganizationID  uint             `json:"organization_id" gorm:"Column:organization_id"`
	CurrentRelease  *FirmwareRelease `json:"release" gorm:"foreignKey:current_release_id"`
	CurrentReleaseID *uint           `json:"current_release_id" gorm:"Column:current_release_id"`
	Active          bool             `json:"active" gorm:"Column:active"`
	FilesPath       string           `json:"files_path" gorm:"Column:files_path"`
	AllowUpdates    bool             `json:"allow_updates" gorm:"Column:allow_updates"`
}

// DeviceMessage model
type DeviceMessage struct {
	UUID         string     `json:"uuid" gorm:"primary_key"`
	CreatedAt    time.Time  `json:"created_at" gorm:"Column:created_at"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"Column:updated_at"`
	Device       *Device    `json:"-" gorm:"foreignkey:DeviceID"`
	DeviceMCU    string     `json:"device_uid" gorm:"Column:device_mcu"`
	DeviceID     uint       `json:"device_id" gorm:"Column:device_id"`
	Message      string     `json:"message" gorm:"Column:message"`
	Published    bool       `json:"published" gorm:"Column:published"`
	PublishedAt  *time.Time `json:"published_at" gorm:"Column:published_at"`
	Republished  bool       `json:"republished" gorm:"Column:republished"`
	Error        bool       `json:"error" gorm:"Column:error"`
	ErrorMessage string     `json:"error_message" gorm:"Column:error_message"`
	SentVia      string     `json:"sent_via" gorm:"Column:sent_via"`
}



