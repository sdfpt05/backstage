package routes

import (
	"example.com/backstage/services/device/api/handlers"
	"example.com/backstage/services/device/internal/service"
	
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// SetupRoutes sets up all the routes for the server
func SetupRoutes(r *gin.Engine, svc service.Service, log *logrus.Logger) {
	// Health check
	r.GET("/health", handlers.HealthCheck)
	
	// API routes
	api := r.Group("/api/v1")
	
	// Device routes
	deviceHandler := handlers.NewDeviceHandler(svc, log)
	devices := api.Group("/devices")
	{
		devices.POST("", deviceHandler.RegisterDevice)
		devices.GET("", deviceHandler.ListDevices)
		devices.GET("/:id", deviceHandler.GetDevice)
		devices.PATCH("/:id/status", deviceHandler.UpdateDeviceStatus)
		devices.POST("/:id/firmware", deviceHandler.AssignFirmware)
		
		// Device messages
		devices.POST("/messages", deviceHandler.ReceiveMessage)
		devices.POST("/messages/batch", deviceHandler.ReceiveBatchMessages) // New batch endpoint
		devices.GET("/:id/messages", deviceHandler.GetDeviceMessages)
		
		// System monitoring endpoints
		devices.GET("/stats/processor", deviceHandler.GetProcessorStats) // New monitoring endpoint
	}
	
	// Organization routes
	orgHandler := handlers.NewOrganizationHandler(svc, log)
	orgs := api.Group("/organizations")
	{
		orgs.POST("", orgHandler.CreateOrganization)
		orgs.GET("", orgHandler.ListOrganizations)
		orgs.GET("/:id", orgHandler.GetOrganization)
		orgs.PUT("/:id", orgHandler.UpdateOrganization)
	}
	
	// Firmware routes
	firmwareHandler := handlers.NewFirmwareHandler(svc, log)
	firmware := api.Group("/firmware")
	{
		firmware.POST("/releases", firmwareHandler.CreateFirmwareRelease)
		firmware.GET("/releases", firmwareHandler.ListFirmwareReleases)
		firmware.GET("/releases/:id", firmwareHandler.GetFirmwareRelease)
		firmware.POST("/releases/:id/activate", firmwareHandler.ActivateFirmwareRelease)
	}
}
