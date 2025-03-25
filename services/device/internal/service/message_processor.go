package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"example.com/backstage/services/device/internal/cache"
	"example.com/backstage/services/device/internal/messaging"
	"example.com/backstage/services/device/internal/models"
	"example.com/backstage/services/device/internal/repository"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// MessageProcessor handles asynchronous message processing
type MessageProcessor struct {
	repo            repository.Repository
	cache           cache.RedisClient
	messagingClient messaging.ServiceBusClient
	log             *logrus.Logger
	workers         int
	queue           chan *models.DeviceMessage
	wg              sync.WaitGroup
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewMessageProcessor creates a new message processor with worker pool
func NewMessageProcessor(
	repo repository.Repository,
	cache cache.RedisClient,
	messagingClient messaging.ServiceBusClient,
	log *logrus.Logger,
	workers int,
) *MessageProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	mp := &MessageProcessor{
		repo:            repo,
		cache:           cache,
		messagingClient: messagingClient,
		log:             log,
		workers:         workers,
		queue:           make(chan *models.DeviceMessage, 10000), // Buffer size
		ctx:             ctx,
		cancel:          cancel,
	}
	
	// Start worker pool
	mp.startWorkers()
	mp.log.Infof("Started message processor with %d workers", workers)
	
	return mp
}

// startWorkers launches the worker goroutines
func (mp *MessageProcessor) startWorkers() {
	for i := 0; i < mp.workers; i++ {
		mp.wg.Add(1)
		go mp.worker(i)
	}
}

// worker processes messages from the queue
func (mp *MessageProcessor) worker(id int) {
	defer mp.wg.Done()
	
	for {
		select {
		case <-mp.ctx.Done():
			mp.log.Debugf("Worker %d shutting down", id)
			return
		case msg := <-mp.queue:
			start := time.Now()
			mp.processMessage(msg)
			mp.log.Debugf("Worker %d processed message in %v", id, time.Since(start))
		}
	}
}

// processMessage handles the actual message processing
func (mp *MessageProcessor) processMessage(message *models.DeviceMessage) {
	ctx := context.Background()
	
	// Try to get device from cache
	cacheKey := fmt.Sprintf("device:%s", message.DeviceMCU)
	var device *models.Device
	var err error
	
	// Try to get from cache first
	cachedData, err := mp.cache.Get(ctx, cacheKey)
	if err == nil {
		var cachedDevice models.Device
		if err := json.Unmarshal([]byte(cachedData), &cachedDevice); err == nil {
			device = &cachedDevice
			mp.log.Debugf("Device found in cache: %s", message.DeviceMCU)
		}
	}
	
	// If not in cache, get from database
	if device == nil {
		device, err = mp.repo.FindDeviceByUID(ctx, message.DeviceMCU)
		if err != nil {
			message.Error = true
			message.ErrorMessage = fmt.Sprintf("Device not found: %s", message.DeviceMCU)
			mp.log.Warnf("Device not found: %s", message.DeviceMCU)
		} else {
			message.DeviceID = device.ID
			message.Device = device
			
			// Update cache in background
			go func(d *models.Device) {
				deviceJSON, err := json.Marshal(d)
				if err == nil {
					mp.cache.Set(context.Background(), cacheKey, string(deviceJSON), 24*time.Hour)
					mp.log.Debugf("Updated device cache for: %s", d.UID)
				} else {
					mp.log.WithError(err).Warnf("Failed to marshal device for cache: %s", d.UID)
				}
			}(device)
		}
	} else {
		message.DeviceID = device.ID
		message.Device = device
	}
	
	// Save message to database
	if err := mp.repo.SaveDeviceMessage(ctx, message); err != nil {
		mp.log.WithError(err).Error("Failed to save message")
		return
	}
	
	// If device was found, publish the message to the message queue
	if !message.Error {
		// Use device UID as the session ID
		sessionID := message.DeviceMCU
		
		// Publish in background to avoid blocking
		go func() {
			pubCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			if err := mp.messagingClient.SendMessage(pubCtx, message, sessionID); err != nil {
				mp.log.WithError(err).Error("Failed to publish message")
				return
			}
			
			// Mark as published
			now := time.Now()
			message.Published = true
			message.PublishedAt = &now
			
			if err := mp.repo.MarkMessageAsPublished(context.Background(), message.UUID); err != nil {
				mp.log.WithError(err).Error("Failed to mark message as published")
			} else {
				mp.log.Debugf("Message marked as published: %s", message.UUID)
			}
		}()
	}
}

// EnqueueMessage adds a message to the queue for processing
func (mp *MessageProcessor) EnqueueMessage(message *models.DeviceMessage) error {
	// Ensure message has UUID
	if message.UUID == "" {
		message.UUID = uuid.New().String()
	}
	
	select {
	case mp.queue <- message:
		return nil
	default:
		// Queue is full, return error
		return errors.New("message queue is full")
	}
}

// Stop gracefully shuts down the processor
func (mp *MessageProcessor) Stop() {
	mp.log.Info("Stopping message processor...")
	mp.cancel()
	mp.wg.Wait()
	mp.log.Info("Message processor stopped")
}

// QueueStats returns current queue statistics
func (mp *MessageProcessor) QueueStats() map[string]interface{} {
	return map[string]interface{}{
		"queue_length":   len(mp.queue),
		"queue_capacity": cap(mp.queue),
		"worker_count":   mp.workers,
	}
}