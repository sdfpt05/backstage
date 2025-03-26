package services

import (
	"context"
	"encoding/json"
	"example.com/backstage/services/sales/internal/cache"
	"example.com/backstage/services/sales/internal/models"
	"example.com/backstage/services/sales/internal/repositories"
	"example.com/backstage/services/sales/internal/search"
	"example.com/backstage/services/sales/internal/tracing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/google/uuid"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// SalesService handles sales-related business logic
type SalesService struct {
	db              *gorm.DB       // Write database
	readOnlyDB      *gorm.DB       // Read-only database
	deviceRepo      *repositories.DeviceRepository
	dmrRepo         *repositories.DeviceMachineRevisionRepository
	mrRepo          *repositories.MachineRevisionRepository
	machineRepo     *repositories.MachineRepository
	tenantRepo      *repositories.TenantRepository
	dsRepo          *repositories.DispenseSessionRepository
	saleRepo        *repositories.SaleRepository
	cache           *cache.RedisCache
	elasticClient   *search.ElasticClient
	tracer          tracing.Tracer
}

// NewSalesService creates a new sales service
func NewSalesService(
	db *gorm.DB,
	readOnlyDB *gorm.DB,
	cache *cache.RedisCache,
	elasticClient *search.ElasticClient,
	tracer tracing.Tracer,
) *SalesService {
	deviceRepo := repositories.NewDeviceRepository(db, readOnlyDB)
	dmrRepo := repositories.NewDeviceMachineRevisionRepository(db, readOnlyDB)
	mrRepo := repositories.NewMachineRevisionRepository(db, readOnlyDB)
	machineRepo := repositories.NewMachineRepository(db, readOnlyDB)
	tenantRepo := repositories.NewTenantRepository(db, readOnlyDB)
	dsRepo := repositories.NewDispenseSessionRepository(db, readOnlyDB)
	saleRepo := repositories.NewSaleRepository(db, readOnlyDB)

	return &SalesService{
		db:              db,
		readOnlyDB:      readOnlyDB,
		deviceRepo:      deviceRepo,
		dmrRepo:         dmrRepo,
		mrRepo:          mrRepo,
		machineRepo:     machineRepo,
		tenantRepo:      tenantRepo,
		dsRepo:          dsRepo,
		saleRepo:        saleRepo,
		cache:           cache,
		elasticClient:   elasticClient,
		tracer:          tracer,
	}
}

// CreateDispenseSession creates a new dispense session and immediately processes it to create a sale
func (s *SalesService) CreateDispenseSession(ctx context.Context, payload *models.SalePayload) (*models.DispenseSession, error) {
	// Create span for session creation
	txn := s.tracer.StartTransaction("create-dispense-session")
	defer s.tracer.EndTransaction(txn)
	
	span := s.tracer.StartSpan("create-dispense-session", txn)
	
	session := &models.DispenseSession{
		ID:                           uuid.New(),
		IdempotencyKey:               payload.IdempotencyKey,
		EventType:                    payload.EventType,
		ExpectedDispense:             payload.EVol,
		RemainingVolume:              payload.RemainingVolume,
		ProductType:                  1, // Default value
		AmountKsh:                    payload.Amount,
		DispenseState:                0, // Default value
		TotalPumpRuntime:             int64(payload.Ms),
		InterpolatedEngineeringVolume: 0, // Default value
		IsProcessed:                  false,
		Time:                         &payload.Time,
		DeviceMcu:                    &payload.Device,
		CreatedAt:                    time.Now(),
		UpdatedAt:                    time.Now(),
	}

	err := s.dsRepo.Create(ctx, session)
	span.End()
	
	if err != nil {
		s.tracer.RecordError(txn, err)
		return nil, errors.Wrap(err, "failed to create dispense session")
	}

	log.Info().
		Str("session_id", session.ID.String()).
		Str("device", *session.DeviceMcu).
		Int32("amount", session.AmountKsh).
		Msg("Dispense session created successfully")

	// Try to process the session immediately
	processSpan := s.tracer.StartSpan("immediate-sale-processing", txn)
	err = s.ProcessDispenseSessionImmediately(ctx, session, payload)
	processSpan.End()
	
	if err != nil {
		// Log the error but don't fail the dispense session creation
		log.Warn().
			Err(err).
			Str("session_id", session.ID.String()).
			Msg("Failed to process dispense session immediately, scheduler will retry")
		
		s.tracer.RecordError(txn, err)
	}

	return session, nil
}

// ProcessDispenseSessionImmediately processes a dispense session immediately after creation
func (s *SalesService) ProcessDispenseSessionImmediately(ctx context.Context, session *models.DispenseSession, payload *models.SalePayload) error {
	// Skip processing if we don't have the required data
	if session.DeviceMcu == nil || session.Time == nil {
		return errors.New("missing required data (device or time) for immediate processing")
	}

	// Convert Unix timestamp to time.Time
	saleTime := time.Unix(int64(*session.Time), 0)

	// Start span for retrieving context data
	contextSpan := s.tracer.StartTransaction("retrieve-sale-context")
	defer s.tracer.EndTransaction(contextSpan)

	// Retrieve sale details from read-only DB
	details, err := s.saleRepo.RetrieveSaleDetails(
		ctx,
		s.deviceRepo,
		s.dmrRepo,
		s.mrRepo,
		s.machineRepo,
		s.tenantRepo,
		*session.DeviceMcu,
		saleTime,
	)
	
	if err != nil {
		s.tracer.RecordError(contextSpan, err)
		return errors.Wrap(err, "failed to retrieve sale details")
	}

	// Determine sale type
	saleType := s.GetSaleType(session.AmountKsh)

	// Start a transaction for sale creation and indexing
	processTxn := s.tracer.StartTransaction("create-and-index-sale")
	defer s.tracer.EndTransaction(processTxn)

	// Execute in a database transaction
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Create the sale
		sale := &models.Sale{
			ID:               uuid.New(),
			MachineRevisionID: details.MachineRevision.ID,
			MachineID:        details.Machine.ID,
			TenantID:         details.Tenant.ID,
			Type:             saleType,
			Quantity:         1,
			Amount:           &session.AmountKsh,
			Position:         0, // Default position
			IsReconciled:     true,
			IsValid:          true,
			Time:             &saleTime,
			DispenseSessionID: session.ID,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		// Insert the sale
		createSpan := s.tracer.StartSpan("create-sale", processTxn)
		if err := tx.Create(sale).Error; err != nil {
			createSpan.End()
			s.tracer.RecordError(processTxn, err)
			return errors.Wrap(err, "failed to create sale")
		}
		createSpan.End()

		// Get the machine's location
		locationSpan := s.tracer.StartSpan("get-machine-location", processTxn)
		location, err := s.machineRepo.GetAddress(ctx, details.Machine.ID)
		locationSpan.End()
		
		if err != nil {
			log.Warn().
				Err(err).
				Str("machine_id", details.Machine.ID.String()).
				Msg("Failed to get machine location")
			location = ""
		}

		// Index the sale in Elasticsearch
		indexSpan := s.tracer.StartSpan("index-sale", processTxn)
		err = s.elasticClient.IndexSale(ctx, sale, details.Machine, location)
		indexSpan.End()
		
		if err != nil {
			s.tracer.RecordError(processTxn, err)
			return errors.Wrap(err, "failed to index sale in Elasticsearch")
		}

		// Mark the dispense session as processed
		markSpan := s.tracer.StartSpan("mark-session-processed", processTxn)
		if err := s.dsRepo.MarkAsProcessed(ctx, session.ID); err != nil {
			markSpan.End()
			s.tracer.RecordError(processTxn, err)
			return errors.Wrap(err, "failed to mark dispense session as processed")
		}
		markSpan.End()

		log.Info().
			Str("session_id", session.ID.String()).
			Str("sale_id", sale.ID.String()).
			Str("device", *session.DeviceMcu).
			Msg("Sale created and indexed successfully")

		return nil
	})

	if err != nil {
		s.tracer.RecordError(processTxn, err)
		return errors.Wrap(err, "transaction failed when processing dispense session")
	}

	return nil
}

// GetSaleType determines the sale type based on parameters
func (s *SalesService) GetSaleType(amount int32) string {
	if amount == 0 {
		return "FREE_VEND"
	}
	return "PAID_VEND"
}

// ProcessDispenseMessage processes a dispense message from Azure Service Bus
func (s *SalesService) ProcessDispenseMessage(ctx context.Context, message *azservicebus.ReceivedMessage, txn *newrelic.Transaction) error {
	// Extract payload details
	payload, err := ExtractDispenseDetails(message)
	if err != nil {
		return errors.Wrap(err, "failed to extract dispense details")
	}

	// Create span for session creation
	span := s.tracer.StartSpan("create-dispense-session", txn)
	
	// Create the dispense session
	session, err := s.CreateDispenseSession(ctx, payload)
	if err != nil {
		span.End()
		return errors.Wrap(err, "failed to create dispense session")
	}
	
	span.End()
	
	log.Info().
		Str("session_id", session.ID.String()).
		Str("device", *session.DeviceMcu).
		Int32("amount", session.AmountKsh).
		Msg("Message processed successfully")

	return nil
}

// ExtractDispenseDetails extracts dispense details from a message
func ExtractDispenseDetails(message *azservicebus.ReceivedMessage) (*models.SalePayload, error) {
	var mainMessage struct {
		EventType string          `json:"ev"`
		Payload   json.RawMessage `json:"payload"`
		Mcu       string          `json:"mcu"`
	}

	if err := json.Unmarshal(message.Body, &mainMessage); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal message")
	}

	var payload models.SalePayload
	if err := json.Unmarshal(mainMessage.Payload, &payload); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal payload")
	}

	// Set the event type from the main message if not set in the payload
	if payload.EventType == "" {
		payload.EventType = mainMessage.EventType
	}

	// Set the device from the MCU if not set in the payload
	if payload.Device == "" {
		payload.Device = mainMessage.Mcu
	}

	return &payload, nil
}

// ReconcileSales processes unprocessed dispense sessions as a fallback mechanism
func (s *SalesService) ReconcileSales(ctx context.Context) error {
	// Start transaction
	txn := s.tracer.StartTransaction("reconcile-sales")
	defer s.tracer.EndTransaction(txn)

	// Get unprocessed dispense sessions
	span := s.tracer.StartSpan("get-unprocessed-sessions", txn)
	sessions, err := s.dsRepo.GetUnprocessed(ctx, 100)
	span.End()

	if err != nil {
		s.tracer.RecordError(txn, err)
		return errors.Wrap(err, "failed to get unprocessed dispense sessions")
	}

	log.Info().Msgf("Found %d unprocessed dispense sessions for reconciliation", len(sessions))

	if len(sessions) == 0 {
		return nil // Nothing to process
	}

	// Process each session
	for _, session := range sessions {
		// Skip sessions without required data
		if session.DeviceMcu == nil || session.Time == nil {
			log.Warn().
				Str("session_id", session.ID.String()).
				Msg("Skipping session with missing data during reconciliation")
			continue
		}

		// Process this session (reusing the same logic as immediate processing)
		// But construct a dummy payload with the minimal required data
		payload := &models.SalePayload{
			Device: *session.DeviceMcu,
			Time:   *session.Time,
		}

		err := s.ProcessDispenseSessionImmediately(ctx, &session, payload)
		if err != nil {
			log.Error().
				Err(err).
				Str("session_id", session.ID.String()).
				Msg("Failed to process dispense session during reconciliation")
			s.tracer.RecordError(txn, err)
			// Continue to next session
		} else {
			log.Info().
				Str("session_id", session.ID.String()).
				Msg("Successfully processed dispense session during reconciliation")
		}
	}

	return nil
}