package services

import (
	"context"
	"example.com/backstage/services/sales/internal/models"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock repositories for testing
type MockDispenseSessionRepository struct {
	mock.Mock
}

func (m *MockDispenseSessionRepository) Create(ctx context.Context, session *models.DispenseSession) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockDispenseSessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.DispenseSession, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*models.DispenseSession), args.Error(1)
}

func (m *MockDispenseSessionRepository) GetUnprocessed(ctx context.Context, limit int) ([]models.DispenseSession, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]models.DispenseSession), args.Error(1)
}

func (m *MockDispenseSessionRepository) MarkAsProcessed(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Mock SaleRepository for testing
type MockSaleRepository struct {
	mock.Mock
}

// Basic test that we can create a dispense session
func TestCreateDispenseSession(t *testing.T) {
	// Create mocks
	mockDsRepo := new(MockDispenseSessionRepository)
	
	// Setup expectations
	mockDsRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.DispenseSession")).Return(nil)
	
	// Create test payload
	payload := &models.SalePayload{
		Amount:         100,
		Device:         "test-device",
		Time:           int32(time.Now().Unix()),
		EventType:      "dispense",
		IdempotencyKey: uuid.New(),
	}

	// Create a minimal SalesService with just what we need for this test
	service := &SalesService{
		dsRepo: mockDsRepo,
	}

	// Call the function
	session, err := service.CreateDispenseSession(context.Background(), payload)
	
	// Verify results
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Equal(t, payload.Device, *session.DeviceMcu)
	require.Equal(t, payload.Amount, session.AmountKsh)
	require.Equal(t, payload.IdempotencyKey, session.IdempotencyKey)
	
	// Verify mocks were called correctly
	mockDsRepo.AssertExpectations(t)
}