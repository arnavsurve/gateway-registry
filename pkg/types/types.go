package types

import (
	"time"
)

// MCPService represents a registered MCP service
type MCPService struct {
	ID           string         `json:"id" gorm:"primaryKey"`
	Name         string         `json:"name" gorm:"not null"`
	Description  string         `json:"description"`
	URL          string         `json:"url" gorm:"not null"`
	Capabilities []Capability   `json:"capabilities" gorm:"foreignKey:ServiceID"`
	Categories   []Category     `json:"categories" gorm:"foreignKey:ServiceID"`
	CreatedAt    time.Time      `json:"created_at" gorm:"autoCreateTime"`
	LastSeen     time.Time      `json:"last_seen"`
	Metadata     []MetadataItem `json:"metadata" gorm:"foreignKey:ServiceID"`
}

// Capability represents a service capability
type Capability struct {
	ID        uint   `json:"-" gorm:"primaryKey"`
	ServiceID string `json:"-"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
}

// Category represents a service category
type Category struct {
	ID        uint   `json:"-" gorm:"primaryKey"`
	ServiceID string `json:"-"`
	Name      string `json:"name"`
}

// MetadataItem represents a service metadata item
type MetadataItem struct {
	ID        uint   `json:"-" gorm:"primaryKey"`
	ServiceID string `json:"-"`
	Key       string `json:"key"`
	Value     string `json:"value"`
}

// ServiceRegistrationRequest represents the incoming registration request
type ServiceRegistrationRequest struct {
	Name         string            `json:"name" binding:"required"`
	Description  string            `json:"description"`
	URL          string            `json:"url" binding:"required"`
	Capabilities map[string]bool   `json:"capabilities" binding:"required"`
	Categories   []string          `json:"categories" binding:"required"`
	Metadata     map[string]string `json:"metadata"`
}

// ServiceResponse represents the outgoing service response
type ServiceResponse struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	URL          string            `json:"url"`
	Capabilities map[string]bool   `json:"capabilities"`
	Categories   []string          `json:"categories"`
	CreatedAt    time.Time         `json:"created_at"`
	LastSeen     time.Time         `json:"last_seen"`
	Metadata     map[string]string `json:"metadata"`
}

// HeartbeatRequest represents a heartbeat request
type HeartbeatRequest struct {
	ServiceID string `json:"service_id" binding:"required"`
}

// Helper functions
func ServiceModelToResponse(service MCPService) ServiceResponse {
	capabilities := make(map[string]bool)
	for _, cap := range service.Capabilities {
		capabilities[cap.Name] = cap.Enabled
	}

	categories := make([]string, len(service.Categories))
	for i, cat := range service.Categories {
		categories[i] = cat.Name
	}

	metadata := make(map[string]string)
	for _, item := range service.Metadata {
		metadata[item.Key] = item.Value
	}

	return ServiceResponse{
		ID:           service.ID,
		Name:         service.Name,
		Description:  service.Description,
		URL:          service.URL,
		Capabilities: capabilities,
		Categories:   categories,
		CreatedAt:    service.CreatedAt,
		LastSeen:     service.LastSeen,
		Metadata:     metadata,
	}
}
