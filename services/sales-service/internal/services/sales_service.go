package services

import (
	"context"
	"sales_service/internal/cache"
	"sales_service/internal/models"
	"sales_service/internal/repositories"
	"sales_service/internal/search"
	"sales_service/internal/tracing"
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
	db              *gorm.DB
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
	cache *cache.RedisCache,
	elasticClient *search.ElasticClient,
	tracer tracing.Tracer,
) *SalesService {
	deviceRepo := repositories.NewDeviceRepository(db)
	dmrRepo := repositories.NewDeviceMachineRevisionRepository(db)
	mrRepo := repositories.NewMachineRevisionRepository(db)
	machineRepo := repositories.NewMachineRepository(db)
	tenantRepo := repositories.NewTenantRepository(db)
	dsRepo := repositories.NewDispenseSessionRepository(db)
	saleRepo := repositories.NewSaleRepository(db)

	return &SalesService{
		db:              db,
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

// CreateDispenseSession creates a new dispense session
func (s *SalesService) CreateDispenseSession(ctx context.Context, payload *models.SalePayload) (*models.DispenseSession, error) {
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
	}

	err := s.dsRepo.Create(ctx, session)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create dispense session")
	}

	log.Info().Str("id", session.ID.String()).Msg("Dispense session created")
	return session, nil
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
		Msg("Dispense session created successfully")

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

// ReconcileSales processes unprocessed dispense sessions and creates sales records
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

	log.Info().Msgf("Found %d unprocessed dispense sessions", len(sessions))

	// Process each session
	for _, session := range sessions {
		// Skip sessions without required data
		if session.DeviceMcu == nil || session.Time == nil {
			log.Warn().
				Str("session_id", session.ID.String()).
				Msg("Skipping session with missing data")
			continue
		}

		// Convert Unix timestamp to time.Time
		saleTime := time.Unix(int64(*session.Time), 0)

		// Get sale details
		span := s.tracer.StartSpan("retrieve-sale-details", txn)
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
		span.End()

		if err != nil {
			log.Error().
				Err(err).
				Str("session_id", session.ID.String()).
				Str("device", *session.DeviceMcu).
				Msg("Failed to retrieve sale details")
			continue
		}

		// Determine sale type
		saleType := s.GetSaleType(session.AmountKsh)

		// Create sale in a transaction
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
				// Set timestamps
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}

			// Insert the sale
			createSpan := s.tracer.StartSpan("create-sale", txn)
			if err := tx.Create(sale).Error; err != nil {
				createSpan.End()
				return errors.Wrap(err, "failed to create sale")
			}
			createSpan.End()

			// Get the machine's location
			locationSpan := s.tracer.StartSpan("get-machine-location", txn)
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
			indexSpan := s.tracer.StartSpan("index-sale", txn)
			err = s.elasticClient.IndexSale(ctx, sale, details.Machine, location)
			indexSpan.End()
			
			if err != nil {
				log.Error().
					Err(err).
					Str("sale_id", sale.ID.String()).
					Msg("Failed to index sale in Elasticsearch")
				// Continue despite indexing error
			}

			// Mark the dispense session as processed
			markSpan := s.tracer.StartSpan("mark-session-processed", txn)
			if err := s.dsRepo.MarkAsProcessed(ctx, session.ID); err != nil {
				markSpan.End()
				return errors.Wrap(err, "failed to mark dispense session as processed")
			}
			markSpan.End()

			return nil
		})

		if err != nil {
			log.Error().
				Err(err).
				Str("session_id", session.ID.String()).
				Msg("Failed to process dispense session")
			s.tracer.RecordError(txn, err)
			continue
		}

		log.Info().
			Str("session_id", session.ID.String()).
			Str("device", *session.DeviceMcu).
			Msg("Successfully processed dispense session")
	}

	return nil
}