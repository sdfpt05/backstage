package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"example.com/backstage/services/device/internal/models"
	"gorm.io/gorm"
)

// Enhanced Firmware Release operations implementation

func (r *repo) CreateFirmwareReleaseExtended(ctx context.Context, release *models.FirmwareReleaseExtended) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}

	// Run in transaction to ensure atomicity
	return gormDB.Transaction(func(tx *gorm.DB) error {
		// First create the base firmware release
		baseRelease := &models.FirmwareRelease{
			FilePath:      release.FilePath,
			ReleaseType:   release.ReleaseType,
			Version:       release.Version,
			Active:        release.Active,
			Size:          release.Size,
			Valid:         release.Valid,
			IsTest:        release.IsTest,
			TestReleaseID: release.TestReleaseID,
			TestDeviceID:  release.TestDeviceID,
			TestPassed:    release.TestPassed,
			FileHash:      release.FileHash,
		}

		if err := tx.Create(baseRelease).Error; err != nil {
			return err
		}

		// Set the ID of the embedded firmware release to the new one
		release.Model = baseRelease.Model

		// Now create the extended release with a reference to the base release
		if err := tx.Create(release).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *repo) UpdateFirmwareReleaseExtended(ctx context.Context, release *models.FirmwareReleaseExtended) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}

	// Run in transaction to ensure atomicity
	return gormDB.Transaction(func(tx *gorm.DB) error {
		// First update the base firmware release
		baseRelease := &models.FirmwareRelease{
			Model:         release.Model,
			FilePath:      release.FilePath,
			ReleaseType:   release.ReleaseType,
			Version:       release.Version,
			Active:        release.Active,
			Size:          release.Size,
			Valid:         release.Valid,
			IsTest:        release.IsTest,
			TestReleaseID: release.TestReleaseID,
			TestDeviceID:  release.TestDeviceID,
			TestPassed:    release.TestPassed,
			FileHash:      release.FileHash,
		}

		if err := tx.Save(baseRelease).Error; err != nil {
			return err
		}

		// Now update the extended release
		if err := tx.Save(release).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *repo) FindFirmwareReleaseExtendedByID(ctx context.Context, id uint) (*models.FirmwareReleaseExtended, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	var release models.FirmwareReleaseExtended
	if err := gormDB.Preload("TestRelease").Preload("TestDevice").Preload("ProductionRelease").First(&release, id).Error; err != nil {
		return nil, err
	}

	return &release, nil
}

func (r *repo) FindFirmwareReleaseByVersion(ctx context.Context, version string) (*models.FirmwareReleaseExtended, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	var release models.FirmwareReleaseExtended
	if err := gormDB.Preload("TestRelease").Preload("TestDevice").Preload("ProductionRelease").
		Where("version = ?", version).First(&release).Error; err != nil {
		return nil, err
	}

	return &release, nil
}

func (r *repo) FindFirmwareReleaseBySemanticVersion(ctx context.Context, major, minor, patch uint) (*models.FirmwareReleaseExtended, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	var release models.FirmwareReleaseExtended
	if err := gormDB.Preload("TestRelease").Preload("TestDevice").Preload("ProductionRelease").
		Where("major_version = ? AND minor_version = ? AND patch_version = ?", major, minor, patch).
		First(&release).Error; err != nil {
		return nil, err
	}

	return &release, nil
}

func (r *repo) ListFirmwareReleasesExtended(ctx context.Context, releaseType models.ReleaseType) ([]*models.FirmwareReleaseExtended, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	var releases []*models.FirmwareReleaseExtended
	query := gormDB.Preload("TestRelease").Preload("ProductionRelease").
		Order("major_version DESC, minor_version DESC, patch_version DESC")

	if releaseType != "" {
		query = query.Where("release_type = ?", releaseType)
	}

	if err := query.Find(&releases).Error; err != nil {
		return nil, err
	}

	return releases, nil
}

func (r *repo) ValidateFirmwareRelease(ctx context.Context, release *models.FirmwareReleaseExtended) (*models.FirmwareReleaseValidation, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	// Create a validation record
	validation := &models.FirmwareReleaseValidation{
		FirmwareReleaseID: release.ID,
		ValidationStatus:  "validating",
		ValidatedAt:       time.Now(),
		ValidatedBy:       "system",
	}

	// Save the initial validation record
	if err := gormDB.Create(validation).Error; err != nil {
		return nil, err
	}

	return validation, nil
}

func (r *repo) GetLatestFirmwareRelease(ctx context.Context, releaseType models.ReleaseType) (*models.FirmwareReleaseExtended, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	var release models.FirmwareReleaseExtended
	query := gormDB.Preload("TestRelease").Preload("ProductionRelease").
		Where("active = ? AND valid = ?", true, true)

	if releaseType != "" {
		query = query.Where("release_type = ?", releaseType)
	}

	if err := query.Order("major_version DESC, minor_version DESC, patch_version DESC").
		First(&release).Error; err != nil {
		return nil, err
	}

	return &release, nil
}

func (r *repo) GetFirmwareManifest(ctx context.Context) (*models.FirmwareManifest, error) {
	gormDB, err := r.db.DB()
	if err != nil {
		return nil, err
	}

	var manifest models.FirmwareManifest
	if err := gormDB.Preload("Releases").Order("created_at DESC").First(&manifest).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// If no manifest exists, create a new one
			manifest = models.FirmwareManifest{
				ManifestVersion:    "1.0.0",
				GeneratedAt:        time.Now(),
				MinimumVersion:     "1.0.0",
				RecommendedVersion: "1.0.0",
			}
			if err := gormDB.Create(&manifest).Error; err != nil {
				return nil, err
			}
			return &manifest, nil
		}
		return nil, err
	}

	return &manifest, nil
}

func (r *repo) UpdateFirmwareManifest(ctx context.Context, manifest *models.FirmwareManifest) error {
	gormDB, err := r.db.DB()
	if err != nil {
		return err
	}

	// Run in transaction to handle associations
	return gormDB.Transaction(func(tx *gorm.DB) error {
		// First clear existing release associations
		if err := tx.Model(manifest).Association("Releases").Clear(); err != nil {
			return fmt.Errorf("failed to clear manifest releases: %w", err)
		}

		// Update manifest fields
		manifest.GeneratedAt = time.Now()
		if err := tx.Save(manifest).Error; err != nil {
			return fmt.Errorf("failed to save manifest: %w", err)
		}

		// Re-add release associations
		if err := tx.Model(manifest).Association("Releases").Replace(manifest.Releases); err != nil {
			return fmt.Errorf("failed to update manifest releases: %w", err)
		}

		return nil
	})
}