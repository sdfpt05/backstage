package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"sales_service/internal/models"
)

// DeviceRepository provides access to device data
type DeviceRepository struct {
	db *gorm.DB
}

// NewDeviceRepository creates a new device repository
func NewDeviceRepository(db *gorm.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

// GetByMCU gets a device by its MCU identifier
func (r *DeviceRepository) GetByMCU(ctx context.Context, mcu string) (*models.Device, error) {
	var device models.Device
	err := r.db.WithContext(ctx).Where("mcu = ?", mcu).First(&device).Error
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device by MCU")
	}
	return &device, nil
}

// DeviceMachineRevisionRepository provides access to device-machine revisions
type DeviceMachineRevisionRepository struct {
	db *gorm.DB
}

// NewDeviceMachineRevisionRepository creates a new repository
func NewDeviceMachineRevisionRepository(db *gorm.DB) *DeviceMachineRevisionRepository {
	return &DeviceMachineRevisionRepository{db: db}
}

// GetActiveAtTime gets the active device-machine revision at a specific time
func (r *DeviceMachineRevisionRepository) GetActiveAtTime(ctx context.Context, deviceID uuid.UUID, saleTime time.Time) (*models.DeviceMachineRevision, error) {
	var revision models.DeviceMachineRevision
	err := r.db.WithContext(ctx).
		Where("device_id = ? AND active = ? AND start <= ? AND (termination IS NULL OR termination > ?)",
			deviceID, true, saleTime, saleTime).
		First(&revision).Error
	if err != nil {
		return nil, errors.Wrap(err, "failed to get active device machine revision")
	}
	return &revision, nil
}

// MachineRevisionRepository provides access to machine revisions
type MachineRevisionRepository struct {
	db *gorm.DB
}

// NewMachineRevisionRepository creates a new repository
func NewMachineRevisionRepository(db *gorm.DB) *MachineRevisionRepository {
	return &MachineRevisionRepository{db: db}
}

// GetActiveAtTime gets the active machine revision at a specific time
func (r *MachineRevisionRepository) GetActiveAtTime(ctx context.Context, deviceMachineRevisionID uuid.UUID, saleTime time.Time) (*models.MachineRevision, error) {
	var revision models.MachineRevision
	err := r.db.WithContext(ctx).
		Where("device_machine_revision_id = ? AND start <= ? AND (terminate IS NULL OR terminate > ?)",
			deviceMachineRevisionID, saleTime, saleTime).
		First(&revision).Error
	if err != nil {
		return nil, errors.Wrap(err, "failed to get active machine revision")
	}
	return &revision, nil
}

// MachineRepository provides access to machine data
type MachineRepository struct {
	db *gorm.DB
}

// NewMachineRepository creates a new repository
func NewMachineRepository(db *gorm.DB) *MachineRepository {
	return &MachineRepository{db: db}
}

// GetByID gets a machine by ID
func (r *MachineRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Machine, error) {
	var machine models.Machine
	err := r.db.WithContext(ctx).First(&machine, id).Error
	if err != nil {
		return nil, errors.Wrap(err, "failed to get machine by ID")
	}
	return &machine, nil
}

// GetAddress gets a machine's location address
func (r *MachineRepository) GetAddress(ctx context.Context, machineID uuid.UUID) (string, error) {
	var location models.Location
	err := r.db.WithContext(ctx).Where("machine_id = ?", machineID).First(&location).Error
	if err != nil {
		return "", errors.Wrap(err, "failed to get machine location")
	}
	return location.Address, nil
}

// TenantRepository provides access to tenant data
type TenantRepository struct {
	db *gorm.DB
}

// NewTenantRepository creates a new repository
func NewTenantRepository(db *gorm.DB) *TenantRepository {
	return &TenantRepository{db: db}
}

// GetByID gets a tenant by ID
func (r *TenantRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error) {
	var tenant models.Tenant
	err := r.db.WithContext(ctx).First(&tenant, id).Error
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant by ID")
	}
	return &tenant, nil
}

// DispenseSessionRepository provides access to dispense session data
type DispenseSessionRepository struct {
	db *gorm.DB
}

// NewDispenseSessionRepository creates a new repository
func NewDispenseSessionRepository(db *gorm.DB) *DispenseSessionRepository {
	return &DispenseSessionRepository{db: db}
}

// Create creates a new dispense session
func (r *DispenseSessionRepository) Create(ctx context.Context, session *models.DispenseSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

// GetByID gets a dispense session by ID
func (r *DispenseSessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.DispenseSession, error) {
	var session models.DispenseSession
	err := r.db.WithContext(ctx).First(&session, id).Error
	if err != nil {
		return nil, errors.Wrap(err, "failed to get dispense session by ID")
	}
	return &session, nil
}

// GetUnprocessed gets unprocessed dispense sessions
func (r *DispenseSessionRepository) GetUnprocessed(ctx context.Context, limit int) ([]models.DispenseSession, error) {
	var sessions []models.DispenseSession
	err := r.db.WithContext(ctx).
		Where("is_processed = ? AND time IS NOT NULL", false).
		Limit(limit).
		Find(&sessions).Error
	if err != nil {
		return nil, errors.Wrap(err, "failed to get unprocessed dispense sessions")
	}
	return sessions, nil
}

// MarkAsProcessed marks a dispense session as processed
func (r *DispenseSessionRepository) MarkAsProcessed(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&models.DispenseSession{}).
		Where("id = ?", id).
		Update("is_processed", true)
	
	if result.Error != nil {
		return errors.Wrap(result.Error, "failed to mark dispense session as processed")
	}
	
	if result.RowsAffected == 0 {
		return errors.New("no dispense session updated")
	}
	
	return nil
}

// SaleRepository provides access to sale data
type SaleRepository struct {
	db *gorm.DB
}

// NewSaleRepository creates a new repository
func NewSaleRepository(db *gorm.DB) *SaleRepository {
	return &SaleRepository{db: db}
}

// Create creates a new sale
func (r *SaleRepository) Create(ctx context.Context, sale *models.Sale) error {
	return r.db.WithContext(ctx).Create(sale).Error
}

// SaleDetails contains all contextual information for a sale
type SaleDetails struct {
	Machine        *models.Machine
	MachineRevision *models.MachineRevision
	Tenant         *models.Tenant
}

// RetrieveSaleDetails gets all details needed for a sale
func (r *SaleRepository) RetrieveSaleDetails(
	ctx context.Context,
	deviceRepo *DeviceRepository,
	dmrRepo *DeviceMachineRevisionRepository,
	mrRepo *MachineRevisionRepository,
	machineRepo *MachineRepository,
	tenantRepo *TenantRepository,
	deviceMCU string,
	saleTime time.Time,
) (*SaleDetails, error) {
	// 1. Get the device
	device, err := deviceRepo.GetByMCU(ctx, deviceMCU)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve device")
	}

	// 2. Get active device machine revision at the sale time
	deviceMachineRevision, err := dmrRepo.GetActiveAtTime(ctx, device.ID, saleTime)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve active device machine revision")
	}

	// 3. Get active machine revision at the sale time
	machineRevision, err := mrRepo.GetActiveAtTime(ctx, deviceMachineRevision.ID, saleTime)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve active machine revision")
	}

	// 4. Get the machine
	machine, err := machineRepo.GetByID(ctx, machineRevision.MachineID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve machine")
	}

	// 5. Get the tenant
	tenant, err := tenantRepo.GetByID(ctx, machineRevision.TenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve tenant")
	}

	return &SaleDetails{
		Machine:        machine,
		MachineRevision: machineRevision,
		Tenant:         tenant,
	}, nil
}