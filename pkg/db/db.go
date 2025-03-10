package db

import (
	"github.com/arnavsurve/gateway-registry/pkg/types"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

// InitDB initializes a database connection and runs migrations
func InitDB() (*gorm.DB, error) {
	dsn := "host=localhost user=postgres password=postgres dbname=gateway port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err = db.AutoMigrate(&types.MCPService{}, &types.Capability{}, &types.Category{}, &types.MetadataItem{}); err != nil {
		return nil, err
	}
	return db, nil
}
