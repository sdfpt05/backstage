package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	
	"example.com/backstage/services/truck/internal/models"
	
	"github.com/google/uuid"
)

// ProcessEvent handles the event processing logic
func (s *service) ProcessEvent(ctx context.Context, event *models.Event) (*models.EventResult, error) {
	// Generate a processing ID
	processingID := uuid.New().String()
	
	// Log event receipt
	s.log.WithFields(map[string]interface{}{
		"event_id":   event.EventID,
		"event_type": event.EventType,
		"device_id":  event.DeviceID,
	}).Info("Processing event")
	
	// Validate the event type
	if !isValidEventType(event.EventType) {
		return &models.EventResult{
			ProcessingID: processingID,
			Status:       "error",
			Message:      fmt.Sprintf("Invalid event type: %s", event.EventType),
		}, nil
	}
	
	// Process the event based on type
	processedEvent := &models.ProcessedEvent{
		ID:           processingID,
		OriginalID:   event.EventID,
		EventType:    event.EventType,
		DeviceID:     event.DeviceID,
		ReceivedAt:   time.Now(),
		ProcessedAt:  time.Now(),
		OriginalTime: event.Timestamp,
		Data:         event.Payload,
		Metadata: map[string]interface{}{
			"processor": "truck",
			"version":   event.Version,
		},
	}
	
	// Apply domain-specific processing
	if err := s.processEventByType(ctx, event, processedEvent); err != nil {
		s.log.WithError(err).Error("Failed to process event by type")
		return &models.EventResult{
			ProcessingID: processingID,
			Status:       "error",
			Message:      "Error processing event",
		}, err
	}
	
	// Store processed event in Elasticsearch
	if err := s.storeProcessedEvent(ctx, processedEvent); err != nil {
		s.log.WithError(err).Error("Failed to store processed event")
		return &models.EventResult{
			ProcessingID: processingID,
			Status:       "error",
			Message:      "Error storing processed event",
		}, err
	}
	
	// Cache event data for quick lookups
	if err := s.cacheEventData(ctx, processedEvent); err != nil {
		s.log.WithError(err).Warn("Failed to cache event data")
	}
	
	return &models.EventResult{
		ProcessingID: processingID,
		Status:       "success",
		Message:      "Event processed successfully",
		Data:         processedEvent,
	}, nil
}

// processEventByType applies domain-specific processing based on event type
func (s *service) processEventByType(ctx context.Context, event *models.Event, processed *models.ProcessedEvent) error {
	// TODO: Implement domain-specific processing logic
	return nil
}

// storeProcessedEvent stores the processed event in Elasticsearch
func (s *service) storeProcessedEvent(ctx context.Context, processed *models.ProcessedEvent) error {
	// Convert to JSON
	data, err := json.Marshal(processed)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	
	// Store in Elasticsearch
	return s.esClient.IndexDocument(ctx, processed.ID, data)
}

// cacheEventData stores event data in Redis for quick lookups
func (s *service) cacheEventData(ctx context.Context, processed *models.ProcessedEvent) error {
	// Create cache key
	key := fmt.Sprintf("event:%s", processed.ID)
	
	// Convert to JSON
	data, err := json.Marshal(processed)
	if err != nil {
		return fmt.Errorf("failed to marshal event for cache: %w", err)
	}
	
	// Store in Redis with expiration (e.g., 24 hours)
	return s.cache.Set(ctx, key, string(data), 24*time.Hour)
}

// isValidEventType checks if an event type is valid for this aggregator
func isValidEventType(eventType string) bool {
	// Valid types for truck aggregator
	validTypes := map[string]bool{
		"truck.location":           true,
		"truck.status":             true,
		"truck.delivery.started":   true,
		"truck.delivery.completed": true,
		"truck.maintenance":        true,
	}
	
	return validTypes[eventType]
}
