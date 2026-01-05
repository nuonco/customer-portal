package models

import (
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// InitDB initializes the database connection.
// It uses DATABASE_URL environment variable for PostgreSQL connection.
// Falls back to local development defaults if not set.
func InitDB() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// Fallback for local development with PostgreSQL (dedicated installer user)
		dsn = "host=localhost user=installer password=installer dbname=installer port=5432 sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Auto-migrate all models
	err = db.AutoMigrate(
		&User{},
		&NuonOrg{},
		&InstallLink{},
		&Install{},
		&AppTheme{},
		&AppHealthCheckConfig{},
	)
	if err != nil {
		return nil, err
	}

	return db, nil
}

// Ping checks the database connection health
func Ping(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}
