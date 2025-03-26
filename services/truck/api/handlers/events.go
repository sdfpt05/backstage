package handlers

import (
	"net/http"
	
	"example.com/backstage/services/truck/internal/models"
	"example.com/backstage/services/truck/internal/service"
	
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// EventHandler handles event processing requests
type EventHandler struct {
	service service.Service
	log     *logrus.Logger
}

// NewEventHandler creates a new EventHandler instance
func NewEventHandler(svc service.Service, log *logrus.Logger) *EventHandler {
	return &EventHandler{
		service: svc,
		log:     log,
	}
}

// ProcessEvent handles event processing requests
func (h *EventHandler) ProcessEvent(c *gin.Context) {
	var event models.Event
	if err := c.ShouldBindJSON(&event); err != nil {
		h.log.WithError(err).Warn("Invalid event format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid event format",
		})
		return
	}
	
	// Process the event
	result, err := h.service.ProcessEvent(c, &event)
	if err != nil {
		h.log.WithError(err).Error("Failed to process event")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to process event",
		})
		return
	}
	
	c.JSON(http.StatusOK, result)
}
