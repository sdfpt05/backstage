package routes

import (
	"example.com/backstage/services/sales/api/handlers"
	"example.com/backstage/services/sales/internal/service"
	
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// SetupRoutes sets up all the routes for the server
func SetupRoutes(r *gin.Engine, svc service.Service, log *logrus.Logger) {
	// Health check
	r.GET("/health", handlers.HealthCheck)
	
	// API routes
	api := r.Group("/api/v1")
	
	// Event handling routes
	events := api.Group("/events")
	eventHandler := handlers.NewEventHandler(svc, log)
	events.POST("", eventHandler.ProcessEvent)
}
