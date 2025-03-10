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

	result := query.Find(&services)
	if result.Error != nil {
		errorResponse(w, "Error finding services", http.StatusInternalServerError)
		return
	}

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

	// Start a transaction
	tx := h.DB.Begin()
	if tx.Error != nil {
		errorResponse(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}

	// Use defer to ensure rollback on panic
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	service := types.MCPService{
		ID:          serviceID,
		Name:        request.Name,
		Description: request.Description,
		URL:         request.URL,
		LastSeen:    now,
		ApiDocs:     request.ApiDocs,
	}

	// Create service in the database
	if err := tx.Create(&service).Error; err != nil {
		tx.Rollback()
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
		if err := tx.Create(&capability).Error; err != nil {
			tx.Rollback()
			errorResponse(w, "Failed to add capability", http.StatusInternalServerError)
			return
		}
	}

	// Add categories
	for _, name := range request.Categories {
		category := types.Category{
			ServiceID: serviceID,
			Name:      name,
		}
		if err := tx.Create(&category).Error; err != nil {
			tx.Rollback()
			errorResponse(w, "Failed to add category", http.StatusInternalServerError)
			return
		}
	}

	// Add metadata
	for key, value := range request.Metadata {
		metadata := types.MetadataItem{
			ServiceID: serviceID,
			Key:       key,
			Value:     value,
		}
		if err := tx.Create(&metadata).Error; err != nil {
			tx.Rollback()
			errorResponse(w, "Failed to add metadata", http.StatusInternalServerError)
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		errorResponse(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	// Retrieve the full service to return
	var createdService types.MCPService
	err := h.DB.Preload("Capabilities").Preload("Categories").Preload("Metadata").First(&createdService, "id = ?", serviceID).Error
	if err != nil {
		errorResponse(w, "Service created but failed to retrieve details", http.StatusInternalServerError)
		return
	}

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

	// Check if service exists before starting transaction
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

	// Start transaction
	tx := h.DB.Begin()
	if tx.Error != nil {
		errorResponse(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}

	// Use defer with recover to ensure rollback on panic
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update service details
	existingService.Name = request.Name
	existingService.Description = request.Description
	existingService.URL = request.URL
	existingService.LastSeen = time.Now()
	existingService.ApiDocs = request.ApiDocs

	if err := tx.Save(&existingService).Error; err != nil {
		tx.Rollback()
		errorResponse(w, "Failed to update service", http.StatusInternalServerError)
		return
	}

	// Update capabilities: remove old ones and add new ones
	if err := tx.Where("service_id = ?", serviceID).Delete(&types.Capability{}).Error; err != nil {
		tx.Rollback()
		errorResponse(w, "Failed to remove old capabilities", http.StatusInternalServerError)
		return
	}

	for name, enabled := range request.Capabilities {
		capability := types.Capability{
			ServiceID: serviceID,
			Name:      name,
			Enabled:   enabled,
		}
		if err := tx.Create(&capability).Error; err != nil {
			tx.Rollback()
			errorResponse(w, "Failed to add capability", http.StatusInternalServerError)
			return
		}
	}

	// Update categories: remove old ones and add new ones
	if err := tx.Where("service_id = ?", serviceID).Delete(&types.Category{}).Error; err != nil {
		tx.Rollback()
		errorResponse(w, "Failed to remove old categories", http.StatusInternalServerError)
		return
	}

	for _, name := range request.Categories {
		category := types.Category{
			ServiceID: serviceID,
			Name:      name,
		}
		if err := tx.Create(&category).Error; err != nil {
			tx.Rollback()
			errorResponse(w, "Failed to add category", http.StatusInternalServerError)
			return
		}
	}

	// Update metadata: remove old ones and add new ones
	if err := tx.Where("service_id = ?", serviceID).Delete(&types.MetadataItem{}).Error; err != nil {
		tx.Rollback()
		errorResponse(w, "Failed to remove old metadata", http.StatusInternalServerError)
		return
	}

	for key, value := range request.Metadata {
		metadata := types.MetadataItem{
			ServiceID: serviceID,
			Key:       key,
			Value:     value,
		}
		if err := tx.Create(&metadata).Error; err != nil {
			tx.Rollback()
			errorResponse(w, "Failed to add metadata", http.StatusInternalServerError)
			return
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		errorResponse(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	// Retrieve the updated service to return (outside transaction)
	var updatedService types.MCPService
	if err := h.DB.Preload("Capabilities").Preload("Categories").Preload("Metadata").
		First(&updatedService, "id = ?", serviceID).Error; err != nil {
		errorResponse(w, "Service updated but failed to retrieve details", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, types.ServiceModelToResponse(updatedService), http.StatusOK)
}

func (h *Handler) DeleteServiceHandler(w http.ResponseWriter, r *http.Request) {
	serviceID := getServiceID(r)
	if serviceID == "" {
		errorResponse(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	// Check if service exists before starting transaction
	var service types.MCPService
	result := h.DB.First(&service, "id = ?", serviceID)
	if result.Error != nil {
		errorResponse(w, "Service not found", http.StatusNotFound)
		return
	}

	// Start transaction
	tx := h.DB.Begin()
	if tx.Error != nil {
		errorResponse(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}

	// Use defer with recover to ensure rollback on panic
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete related records first
	if err := tx.Where("service_id = ?", serviceID).Delete(&types.Capability{}).Error; err != nil {
		tx.Rollback()
		errorResponse(w, "Failed to delete capabilities", http.StatusInternalServerError)
		return
	}

	if err := tx.Where("service_id = ?", serviceID).Delete(&types.Category{}).Error; err != nil {
		tx.Rollback()
		errorResponse(w, "Failed to delete categories", http.StatusInternalServerError)
		return
	}

	if err := tx.Where("service_id = ?", serviceID).Delete(&types.MetadataItem{}).Error; err != nil {
		tx.Rollback()
		errorResponse(w, "Failed to delete metadata", http.StatusInternalServerError)
		return
	}

	// Delete the service
	if err := tx.Delete(&service).Error; err != nil {
		tx.Rollback()
		errorResponse(w, "Failed to delete service", http.StatusInternalServerError)
		return
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		errorResponse(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

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
	result := h.DB.Preload("Capabilities").Preload("Categories").Preload("Metadata").
		Where("name ILIKE ? OR description ILIKE ?", "%"+query+"%", "%"+query+"%").
		Find(&services)

	if result.Error != nil {
		errorResponse(w, "Error searching for services", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	var responses []types.ServiceResponse
	for _, service := range services {
		responses = append(responses, types.ServiceModelToResponse(service))
	}

	jsonResponse(w, responses, http.StatusOK)
}
