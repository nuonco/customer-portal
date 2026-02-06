// Package testutil provides database testing utilities.
package testutil

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

var (
	testDB     *gorm.DB
	testDBOnce sync.Once
	testDBErr  error
)

// GetTestDatabaseURL returns the database URL for testing.
// It reads from TEST_DATABASE_URL environment variable or returns empty string.
func GetTestDatabaseURL() string {
	return os.Getenv("TEST_DATABASE_URL")
}

// RequireTestDB returns a test database connection, skipping the test if unavailable.
// This is the recommended way to get a database connection in integration tests.
func RequireTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	url := GetTestDatabaseURL()
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	db, err := GetTestDB()
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	return db
}

// GetTestDB returns a singleton test database connection.
// Call this once per test run to get a shared database connection.
// Returns an error if TEST_DATABASE_URL is not set or connection fails.
func GetTestDB() (*gorm.DB, error) {
	testDBOnce.Do(func() {
		url := GetTestDatabaseURL()
		if url == "" {
			testDBErr = fmt.Errorf("TEST_DATABASE_URL environment variable not set")
			return
		}

		db, err := gorm.Open(postgres.Open(url), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			testDBErr = fmt.Errorf("failed to connect to test database: %w", err)
			return
		}

		// Run migrations
		if err := runMigrations(db); err != nil {
			testDBErr = fmt.Errorf("failed to run migrations: %w", err)
			return
		}

		testDB = db
	})

	return testDB, testDBErr
}

// runMigrations runs the database migrations for testing.
func runMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.NuonOrg{},
		&models.OrgMember{},
		&models.OrgInvitation{},
		&models.InstallLink{},
		&models.Install{},
		&models.AppTheme{},
		&models.CustomerAuthConfig{},
		&models.AppInputConfig{},
		&models.AppHealthCheckConfig{},
		&models.GitHubRepoConfig{},
		&models.TemplateOverride{},
		&models.AssetOverride{},
	)
}

// TestTransaction wraps a test function in a database transaction that is rolled back.
// This ensures test isolation without affecting the actual database state.
//
// Usage:
//
//	func TestMyFeature(t *testing.T) {
//	    testutil.TestTransaction(t, func(tx *gorm.DB) {
//	        // Use tx for all database operations
//	        // Changes will be rolled back after the function returns
//	    })
//	}
func TestTransaction(t *testing.T, fn func(tx *gorm.DB)) {
	t.Helper()

	db := RequireTestDB(t)

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
		tx.Rollback()
	}()

	fn(tx)
}

// CleanupTable removes all rows from a table for testing.
// Use sparingly - prefer TestTransaction for isolation.
func CleanupTable(db *gorm.DB, tableName string) error {
	return db.Exec("DELETE FROM " + tableName).Error
}

// CleanupAllTables removes all data from all tables.
// This is useful for setting up a clean slate before integration tests.
func CleanupAllTables(db *gorm.DB) error {
	tables := []string{
		"asset_overrides",
		"template_overrides",
		"github_repo_configs",
		"health_check_configs",
		"app_input_configs",
		"customer_auth_configs",
		"app_themes",
		"installs",
		"install_links",
		"org_invitations",
		"org_members",
		"nuon_orgs",
		"users",
	}

	for _, table := range tables {
		if err := db.Exec("DELETE FROM " + table).Error; err != nil {
			return fmt.Errorf("failed to cleanup %s: %w", table, err)
		}
	}

	return nil
}

// CreateTestData creates a set of test data for integration testing.
// Returns the created entities for use in test assertions.
type TestDataSet struct {
	Vendor     *models.User
	Customer   *models.User
	Org        *models.NuonOrg
	Membership *models.OrgMember
	Link       *models.InstallLink
	Install    *models.Install
	Theme      *models.AppTheme
}

// CreateTestDataSet creates a complete set of related test entities in the database.
func CreateTestDataSet(db *gorm.DB) (*TestDataSet, error) {
	vendor := NewTestVendor()
	if err := db.Create(vendor).Error; err != nil {
		return nil, fmt.Errorf("failed to create vendor: %w", err)
	}

	customer := NewTestCustomer()
	if err := db.Create(customer).Error; err != nil {
		return nil, fmt.Errorf("failed to create customer: %w", err)
	}

	org := NewTestOrg(OrgOptions{UserID: vendor.ID})
	if err := db.Create(org).Error; err != nil {
		return nil, fmt.Errorf("failed to create org: %w", err)
	}

	membership := NewTestOrgMember(OrgMemberOptions{
		UserID: vendor.ID,
		OrgID:  org.ID,
	})
	if err := db.Create(membership).Error; err != nil {
		return nil, fmt.Errorf("failed to create membership: %w", err)
	}

	link := NewTestInstallLink(InstallLinkOptions{
		OrgID:  org.ID,
		UserID: vendor.ID,
	})
	if err := db.Create(link).Error; err != nil {
		return nil, fmt.Errorf("failed to create install link: %w", err)
	}

	install := NewTestInstall(InstallOptions{
		OrgID:             org.ID,
		UserID:            customer.ID,
		CreatedByVendorID: vendor.ID,
		InstallLinkID:     link.ID,
	})
	if err := db.Create(install).Error; err != nil {
		return nil, fmt.Errorf("failed to create install: %w", err)
	}

	theme := NewTestTheme(ThemeOptions{OrgID: org.ID})
	if err := db.Create(theme).Error; err != nil {
		return nil, fmt.Errorf("failed to create theme: %w", err)
	}

	return &TestDataSet{
		Vendor:     vendor,
		Customer:   customer,
		Org:        org,
		Membership: membership,
		Link:       link,
		Install:    install,
		Theme:      theme,
	}, nil
}

// SeedMinimalData creates the minimum data needed for basic tests.
func SeedMinimalData(db *gorm.DB) (*models.User, *models.NuonOrg, error) {
	user := NewTestVendor()
	if err := db.Create(user).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to create user: %w", err)
	}

	org := NewTestOrg(OrgOptions{UserID: user.ID})
	if err := db.Create(org).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to create org: %w", err)
	}

	membership := NewTestOrgMember(OrgMemberOptions{
		UserID: user.ID,
		OrgID:  org.ID,
	})
	if err := db.Create(membership).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to create membership: %w", err)
	}

	return user, org, nil
}
