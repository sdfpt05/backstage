package models

import (
	"time"
)

// Event represents an event message from IoT devices
type Event struct {
	EventID   string                 `json:"event_id"`
	EventType string                 `json:"event_type"`
	DeviceID  string                 `json:"device_id"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
	Payload   map[string]interface{} `json:"payload"`
}

// EventResult represents the result of event processing
type EventResult struct {
	ProcessingID string      `json:"processing_id"`
	Status       string      `json:"status"`
	Message      string      `json:"message"`
	Data         interface{} `json:"data,omitempty"`
}

// ProcessedEvent represents an event that has been processed and is ready for Elasticsearch
type ProcessedEvent struct {
	ID           string                 `json:"id"`
	OriginalID   string                 `json:"original_id"`
	EventType    string                 `json:"event_type"`
	DeviceID     string                 `json:"device_id"`
	ReceivedAt   time.Time              `json:"received_at"`
	ProcessedAt  time.Time              `json:"processed_at"`
	OriginalTime time.Time              `json:"original_time"`
	Data         map[string]interface{} `json:"data"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
