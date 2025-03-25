package service

import (
	"context"
	
	"example.com/backstage/services/sales/internal/cache"
	"example.com/backstage/services/sales/internal/messaging"
	"example.com/backstage/services/sales/internal/repository"
	"example.com/backstage/services/sales/internal/elasticsearch"
	"example.com/backstage/services/sales/internal/models"
	
	"github.com/sirupsen/logrus"
)

// Service defines the business logic operations
type Service interface {
	// ProcessEvent processes an incoming event
	ProcessEvent(ctx context.Context, event *models.Event) (*models.EventResult, error)
}

// service is an implementation of the Service interface
type service struct {
	repo           repository.Repository
	cache          cache.RedisClient
	messagingClient messaging.ServiceBusClient
	esClient       elasticsearch.Client
	log            *logrus.Logger
}

// NewService creates a new service instance
func NewService(
	repo repository.Repository, 
	cache cache.RedisClient,
	messagingClient messaging.ServiceBusClient,
	esClient elasticsearch.Client,
	log *logrus.Logger,
) Service {
	return &service{
		repo:           repo,
		cache:          cache,
		messagingClient: messagingClient,
		esClient:       esClient,
		log:            log,
	}
}


