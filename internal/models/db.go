package models

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// getEnvOrDefault returns the environment variable value or a default.
func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// buildDSN constructs a PostgreSQL connection string from environment variables.
func buildDSN() (string, error) {
	// First check for DATABASE_URL (used in local dev)
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn, nil
	}

	// Build from individual variables (used in k8s)
	host := getEnvOrDefault("DB_HOST", "localhost")
	name := getEnvOrDefault("DB_NAME", "customer_dashboard")
	user := getEnvOrDefault("DB_USER", "customer_dashboard")
	port := getEnvOrDefault("DB_PORT", "5432")
	sslMode := getEnvOrDefault("DB_SSL_MODE", "disable")
	region := getEnvOrDefault("DB_REGION", "us-west-2")
	useIAM := os.Getenv("DB_USE_IAM") == "true"

	var password string
	if useIAM {
		// Use AWS RDS IAM authentication
		ctx := context.Background()
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
		if err != nil {
			return "", fmt.Errorf("unable to load AWS config: %w", err)
		}

		endpoint := fmt.Sprintf("%s:%s", host, port)
		token, err := auth.BuildAuthToken(ctx, endpoint, region, user, cfg.Credentials)
		if err != nil {
			return "", fmt.Errorf("unable to build RDS auth token: %w", err)
		}
		password = token
	} else {
		password = os.Getenv("DB_PASSWORD")
	}

	if password != "" {
		return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
			host, user, password, name, port, sslMode), nil
	}

	return fmt.Sprintf("host=%s user=%s dbname=%s port=%s sslmode=%s",
		host, user, name, port, sslMode), nil
}

// InitDB initializes the database connection.
// Supports DATABASE_URL or individual DB_* environment variables.
// When DB_USE_IAM=true, uses AWS RDS IAM authentication.
func InitDB() (*gorm.DB, error) {
	dsn, err := buildDSN()
	if err != nil {
		return nil, fmt.Errorf("failed to build DSN: %w", err)
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
		&CustomerAuthConfig{},
		&AppInputConfig{},
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
