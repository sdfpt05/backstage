package models

import (
	"time"
)

// SemanticVersion represents a semantic version with major, minor, and patch components
type SemanticVersion struct {
	Major      uint `json:"major"`
	Minor      uint `json:"minor"`
	Patch      uint `json:"patch"`
	PreRelease string `json:"pre_release,omitempty"`
	Build      string `json:"build,omitempty"`
}

// FirmwareReleaseExtended enhances the FirmwareRelease model with additional fields
// for semantic versioning and signing
type FirmwareReleaseExtended struct {
	FirmwareRelease            // Embed the base model
	MajorVersion      uint     `json:"major_version" gorm:"Column:major_version"`
	MinorVersion      uint     `json:"minor_version" gorm:"Column:minor_version"`
	PatchVersion      uint     `json:"patch_version" gorm:"Column:patch_version"`
	PreReleaseVersion string   `json:"pre_release_version" gorm:"Column:pre_release_version"`
	BuildMetadata     string   `json:"build_metadata" gorm:"Column:build_metadata"`
	Signature         string   `json:"signature" gorm:"Column:signature"`
	SignatureAlgorithm string  `json:"signature_algorithm" gorm:"Column:signature_algorithm;default:'ecdsa-secp256r1'"`
	SignedAt          *time.Time `json:"signed_at" gorm:"Column:signed_at"`
	SignedBy          string   `json:"signed_by" gorm:"Column:signed_by"`
	CertificateID     string   `json:"certificate_id" gorm:"Column:certificate_id"`
	RequiredVersion   string   `json:"required_version" gorm:"Column:required_version"`
	ReleaseNotes      string   `json:"release_notes" gorm:"Column:release_notes;type:text"`
	ChangesFrom       *uint    `json:"changes_from" gorm:"Column:changes_from"`
	ProductionRelease *FirmwareRelease `json:"production_release" gorm:"foreignKey:ProductionReleaseID"`
	ProductionReleaseID *uint  `json:"production_release_id" gorm:"Column:production_release_id"`
	DeltaPackages     bool     `json:"delta_packages" gorm:"Column:delta_packages"`
	RequiresFullImage bool     `json:"requires_full_image" gorm:"Column:requires_full_image"`
}

// FirmwareReleaseValidation represents validation results for a firmware release
type FirmwareReleaseValidation struct {
	Model
	FirmwareRelease   *FirmwareRelease `json:"firmware_release" gorm:"foreignKey:FirmwareReleaseID"`
	FirmwareReleaseID uint             `json:"firmware_release_id" gorm:"Column:firmware_release_id"`
	ValidationStatus  string           `json:"validation_status" gorm:"Column:validation_status"`
	ValidationErrors  string           `json:"validation_errors" gorm:"Column:validation_errors;type:text"`
	ValidatedAt       time.Time        `json:"validated_at" gorm:"Column:validated_at"`
	ValidatedBy       string           `json:"validated_by" gorm:"Column:validated_by"`
	SignatureValid    bool             `json:"signature_valid" gorm:"Column:signature_valid"`
	HashValid         bool             `json:"hash_valid" gorm:"Column:hash_valid"`
	SizeValid         bool             `json:"size_valid" gorm:"Column:size_valid"`
	VersionValid      bool             `json:"version_valid" gorm:"Column:version_valid"`
}

// FirmwareManifest provides metadata for available firmware
type FirmwareManifest struct {
	Model
	Releases          []FirmwareRelease `json:"releases" gorm:"many2many:manifest_releases"`
	ManifestVersion   string            `json:"manifest_version" gorm:"Column:manifest_version"`
	GeneratedAt       time.Time         `json:"generated_at" gorm:"Column:generated_at"`
	Signature         string            `json:"signature" gorm:"Column:signature"`
	SignatureAlgorithm string           `json:"signature_algorithm" gorm:"Column:signature_algorithm"`
	MinimumVersion    string            `json:"minimum_version" gorm:"Column:minimum_version"`
	RecommendedVersion string           `json:"recommended_version" gorm:"Column:recommended_version"`
}