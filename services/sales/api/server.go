package api

import (
	"context"
	"fmt"
	"net/http"

	"example.com/backstage/services/sales/api/routes"
	"example.com/backstage/services/sales/api/middleware"
	"example.com/backstage/services/sales/config"
	"example.com/backstage/services/sales/internal/cache"
	"example.com/backstage/services/sales/internal/database"
	"example.com/backstage/services/sales/internal/messaging"
	"example.com/backstage/services/sales/internal/repository"
	"example.com/backstage/services/sales/internal/service"
	"example.com/backstage/services/sales/internal/elasticsearch"
	
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// Server represents the HTTP server
type Server struct {
	router     *gin.Engine
	config     *config.Config
	httpServer *http.Server
	log        *logrus.Logger
}

// NewServer creates a new HTTP server
func NewServer(
	config *config.Config, 
	log *logrus.Logger, 
	nrApp *newrelic.Application,
	db database.DB,
	redisClient cache.RedisClient,
	sbClient messaging.ServiceBusClient,
	esClient elasticsearch.Client,
) *Server {
	// Set Gin mode
	gin.SetMode(config.Server.Mode)
	
	// Create router
	router := gin.New()
	
	// Create repositories
	repo := repository.NewRepository(db)
	
	// Create service layer
	svc := service.NewService(repo, redisClient, sbClient, esClient, log)
	
	// Set up middleware
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(log))
	
	// Add New Relic middleware if enabled
	if nrApp != nil {
		router.Use(middleware.NewRelicMiddleware(nrApp))
	}
	
	// Set up routes
	routes.SetupRoutes(router, svc, log)
	
	return &Server{
		router: router,
		config: config,
		log:    log,
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%d", config.Server.Port),
			Handler: router,
		},
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.log.Infof("Starting server on port %d", s.config.Server.Port)
	return s.httpServer.ListenAndServe()
}

// Shutdown stops the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
