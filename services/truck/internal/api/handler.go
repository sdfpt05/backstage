package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"example.com/backstage/services/truck/internal/service"
)

// Handler defines the API handler
type Handler struct {
	operationService      service.OperationService
	operationGroupService service.OperationGroupService
	operationEventService service.OperationEventService
}

// NewHandler creates a new API handler
func NewHandler(
	operationService service.OperationService,
	operationGroupService service.OperationGroupService,
	operationEventService service.OperationEventService,
) *Handler {
	return &Handler{
		operationService:      operationService,
		operationGroupService: operationGroupService,
		operationEventService: operationEventService,
	}
}

// RegisterRoutes registers API routes
func (h *Handler) RegisterRoutes(r *mux.Router) {
	// Operation routes
	r.HandleFunc("/ops/op/{device_uid}", h.GetActiveOperation).Methods(http.MethodGet)
	r.HandleFunc("/ops/op/{device_uid}/events", h.RecordOperationEvent).Methods(http.MethodPost)
	
	// Operation group routes
	r.HandleFunc("/ops/opg/{truck_uid}", h.GetActiveOperationGroup).Methods(http.MethodGet)
	r.HandleFunc("/ops/opg/{truck_uid}/events", h.RecordOperationEvent).Methods(http.MethodPost)
}

// GetActiveOperation gets the active operation for a device
func (h *Handler) GetActiveOperation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceMCU := vars["device_uid"]
	
	if deviceMCU == "" {
		WriteError(w, ErrInvalidRequest)
		return
	}
	
	operation, err := h.operationService.FindActiveByDeviceMCU(r.Context(), deviceMCU)
	if err != nil {
		WriteError(w, ErrNotFound)
		return
	}
	
	if operation == nil {
		WriteError(w, ErrNotFound)
		return
	}
	
	writeJSONResponse(w, http.StatusOK, operation)
}

// GetActiveOperationGroup gets the active operation group for a truck
func (h *Handler) GetActiveOperationGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	truckMCU := vars["truck_uid"]
	
	if truckMCU == "" {
		WriteError(w, ErrInvalidRequest)
		return
	}
	
	group, err := h.operationGroupService.FindActiveByTransportMCU(r.Context(), truckMCU)
	if err != nil {
		WriteError(w, ErrNotFound)
		return
	}
	
	if group == nil {
		WriteError(w, ErrNotFound)
		return
	}
	
	writeJSONResponse(w, http.StatusOK, group)
}

// RecordOperationEvent records an operation event
func (h *Handler) RecordOperationEvent(w http.ResponseWriter, r *http.Request) {
	var req service.RecordEventRequest
	
	// Parse the request body
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, NewValidationError("Invalid request body"))
		return
	}
	
	// Get device MCU from URL
	vars := mux.Vars(r)
	deviceMCU := vars["device_uid"]
	truckMCU := vars["truck_uid"]
	
	// Use the appropriate MCU
	if deviceMCU != "" {
		req.DeviceMCU = deviceMCU
	} else if truckMCU != "" {
		req.DeviceMCU = truckMCU
	} else {
		WriteError(w, NewValidationError("Device or truck ID is required"))
		return
	}
	
	// Record the event
	if err := h.operationEventService.RecordEvent(r.Context(), &req); err != nil {
		logrus.WithError(err).Error("Failed to record operation event")
		WriteError(w, ErrInternalServer)
		return
	}
	
	// Return success
	writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// writeJSONResponse writes a JSON response
func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logrus.WithError(err).Error("Failed to encode JSON response")
	}
}