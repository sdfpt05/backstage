package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/backstage/services/truck/api"
	"example.com/backstage/services/truck/config"
	"example.com/backstage/services/truck/internal/cache"
	"example.com/backstage/services/truck/internal/database"
	"example.com/backstage/services/truck/internal/messaging"
	"example.com/backstage/services/truck/internal/telemetry"

	"github.com/sirupsen/logrus"
)

func main() {
	// Initialize logger
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.InfoLevel)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize NewRelic
	nrApp, err := telemetry.InitNewRelic(cfg.NewRelic)
	if err != nil {
		log.Warnf("Failed to initialize New Relic: %v", err)
	}

	// Connect to database
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize Redis cache
	redisClient, err := cache.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Warnf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// Initialize Azure Service Bus
	sbClient, err := messaging.NewServiceBusClient(cfg.ServiceBus, "truck-consumer")
	if err != nil {
		log.Fatalf("Failed to connect to Azure Service Bus: %v", err)
	}
	defer sbClient.Close()

	// Initialize Elasticsearch
	esClient, err := elasticsearch.NewClient(cfg.Elasticsearch)
	if err != nil {
		log.Fatalf("Failed to connect to Elasticsearch: %v", err)
	}

	// Initialize and start the server
	server := api.NewServer(cfg, log, nrApp, db, redisClient, sbClient, esClient)
	go func() {
		if err := server.Start(); err != nil {
			log.Infof("Server stopped: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down server...")

	// Shutdown gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Info("Server successfully shutdown")
}
