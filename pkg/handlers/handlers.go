package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/arnavsurve/gateway-registry/pkg/types"
	"gorm.io/gorm"
)

// TODO: refactor handlers into individual files

type Handler struct {
	DB *gorm.DB
}

// Helper functions
func errorResponse(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	response := map[string]string{"error": message}
	json.NewEncoder(w).Encode(response)
}

func jsonResponse(w http.ResponseWriter, data any, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func getServiceID(r *http.Request) string {
	vars := mux.Vars(r)
	return vars["id"]
}

func (h *Handler) ListServicesHandler(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	var services []types.MCPService
	query := h.DB.Preload("Capabilities").Preload("Categories").Preload("Metadata")

	if category != "" {
		var serviceIDs []string
		h.DB.Model(&types.Category{}).Where("name = ?", category).Pluck("service_id", &serviceIDs)

		if len(serviceIDs) > 0 {
			query = query.Where("id IN ?", serviceIDs)
		} else {
			jsonResponse(w, []types.ServiceResponse{}, http.StatusOK)
			return
		}
	}

	query.Find(&services)

	// Convert to response format
	var responses []types.ServiceResponse
	for _, service := range services {
		responses = append(responses, types.ServiceModelToResponse(service))
	}

	jsonResponse(w, responses, http.StatusOK)
}

func (h *Handler) CreateServiceHandler(w http.ResponseWriter, r *http.Request) {
	var request types.ServiceRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		errorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if request.Name == "" || request.URL == "" ||
		request.Capabilities == nil || request.Categories == nil {
		errorResponse(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	serviceID := uuid.New().String()
	now := time.Now()

	service := types.MCPService{
		ID:          serviceID,
		Name:        request.Name,
		Description: request.Description,
		URL:         request.URL,
		LastSeen:    now,
		ApiDocs:     request.ApiDocs,
	}

	// Create service in the database
	result := h.DB.Create(&service)
	if result.Error != nil {
		errorResponse(w, "Failed to register service", http.StatusInternalServerError)
		return
	}

	// Add capabilities
	for name, enabled := range request.Capabilities {
		capability := types.Capability{
			ServiceID: serviceID,
			Name:      name,
			Enabled:   enabled,
		}
		h.DB.Create(&capability)
	}

	// Add categories
	for _, name := range request.Categories {
		category := types.Category{
			ServiceID: serviceID,
			Name:      name,
		}
		h.DB.Create(&category)
	}

	// Add metadata
	for key, value := range request.Metadata {
		metadata := types.MetadataItem{
			ServiceID: serviceID,
			Key:       key,
			Value:     value,
		}
		h.DB.Create(&metadata)
	}

	// Retrieve the full service to return
	var createdService types.MCPService
	h.DB.Preload("Capabilities").Preload("Categories").Preload("Metadata").First(&createdService, "id = ?", serviceID)

	jsonResponse(w, types.ServiceModelToResponse(createdService), http.StatusCreated)
}

func (h *Handler) GetServiceHandler(w http.ResponseWriter, r *http.Request) {
	serviceID := getServiceID(r)
	if serviceID == "" {
		errorResponse(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	var service types.MCPService
	result := h.DB.Preload("Capabilities").Preload("Categories").Preload("Metadata").First(&service, "id = ?", serviceID)
	if result.Error != nil {
		errorResponse(w, "Service not found", http.StatusNotFound)
		return
	}

	jsonResponse(w, types.ServiceModelToResponse(service), http.StatusOK)
}

func (h *Handler) UpdateServiceHandler(w http.ResponseWriter, r *http.Request) {
	serviceID := getServiceID(r)
	if serviceID == "" {
		errorResponse(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	var existingService types.MCPService
	result := h.DB.First(&existingService, "id = ?", serviceID)
	if result.Error != nil {
		errorResponse(w, "Service not found", http.StatusNotFound)
		return
	}

	var request types.ServiceRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		errorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update service details
	existingService.Name = request.Name
	existingService.Description = request.Description
	existingService.URL = request.URL
	existingService.LastSeen = time.Now()
	existingService.ApiDocs = request.ApiDocs

	h.DB.Save(&existingService)

	// Update capabilities: remove old ones and add new ones
	h.DB.Where("service_id = ?", serviceID).Delete(&types.Capability{})
	for name, enabled := range request.Capabilities {
		capability := types.Capability{
			ServiceID: serviceID,
			Name:      name,
			Enabled:   enabled,
		}
		h.DB.Create(&capability)
	}

	// Update categories: remove old ones and add new ones
	h.DB.Where("service_id = ?", serviceID).Delete(&types.Category{})
	for _, name := range request.Categories {
		category := types.Category{
			ServiceID: serviceID,
			Name:      name,
		}
		h.DB.Create(&category)
	}

	// Update metadata: remove old ones and add new ones
	h.DB.Where("service_id = ?", serviceID).Delete(&types.MetadataItem{})
	for key, value := range request.Metadata {
		metadata := types.MetadataItem{
			ServiceID: serviceID,
			Key:       key,
			Value:     value,
		}
		h.DB.Create(&metadata)
	}

	// Retrieve the updated service to return
	var updatedService types.MCPService
	h.DB.Preload("Capabilities").Preload("Categories").Preload("Metadata").First(&updatedService, "id = ?", serviceID)

	jsonResponse(w, types.ServiceModelToResponse(updatedService), http.StatusOK)
}

func (h *Handler) DeleteServiceHandler(w http.ResponseWriter, r *http.Request) {
	serviceID := getServiceID(r)
	if serviceID == "" {
		errorResponse(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	var service types.MCPService
	result := h.DB.First(&service, "id = ?", serviceID)
	if result.Error != nil {
		errorResponse(w, "Service not found", http.StatusNotFound)
		return
	}

	// Delete related records
	h.DB.Where("service_id = ?", serviceID).Delete(&types.Capability{})
	h.DB.Where("service_id = ?", serviceID).Delete(&types.Category{})
	h.DB.Where("service_id = ?", serviceID).Delete(&types.MetadataItem{})

	// Delete the service
	h.DB.Delete(&service)

	jsonResponse(w, map[string]string{"message": "Service unregistered"}, http.StatusOK)
}

func (h *Handler) HeartbeatHandler(w http.ResponseWriter, r *http.Request) {
	serviceID := getServiceID(r)
	if serviceID == "" {
		errorResponse(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	var service types.MCPService
	result := h.DB.First(&service, "id = ?", serviceID)
	if result.Error != nil {
		errorResponse(w, "Service not found", http.StatusNotFound)
		return
	}

	// Update last seen time
	service.LastSeen = time.Now()
	h.DB.Save(&service)

	jsonResponse(w, map[string]string{"message": "Heartbeat received"}, http.StatusOK)
}

func (h *Handler) SearchServicesHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		errorResponse(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	var services []types.MCPService
	h.DB.Preload("Capabilities").Preload("Categories").Preload("Metadata").
		Where("name ILIKE ? OR description ILIKE ?", "%"+query+"%", "%"+query+"%").
		Find(&services)

	// Convert to response format
	var responses []types.ServiceResponse
	for _, service := range services {
		responses = append(responses, types.ServiceModelToResponse(service))
	}

	jsonResponse(w, responses, http.StatusOK)
}
