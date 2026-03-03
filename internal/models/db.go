package models

import (
	"context"
	"fmt"
	"os"
	"strings"

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

	// IMPORTANT: Prepare install_links.name column BEFORE AutoMigrate
	// This adds the column as nullable and populates existing rows,
	// so AutoMigrate can safely add the NOT NULL constraint.
	if err := prepareInstallLinkNameColumn(db); err != nil {
		return nil, fmt.Errorf("install link name preparation failed: %w", err)
	}

	// Run install_link_id nullable migration (make install_link_id nullable for published-app installs)
	if err := runInstallLinkIDNullableMigration(db); err != nil {
		return nil, fmt.Errorf("install_link_id nullable migration failed: %w", err)
	}

	// Run created_by_vendor_id nullable migration (make it nullable for published-app installs)
	if err := runCreatedByVendorIDNullableMigration(db); err != nil {
		return nil, fmt.Errorf("created_by_vendor_id nullable migration failed: %w", err)
	}

	// Migrate customer_account_invites from token-based to email-based schema
	if err := runInviteEmailMigration(db); err != nil {
		return nil, fmt.Errorf("invite email migration failed: %w", err)
	}

	// Auto-migrate all models
	err = db.AutoMigrate(
		&User{},
		&NuonOrg{},
		&OrgMember{},
		&OrgInvitation{},
		&InstallLink{},
		&Install{},
		&PublishedApp{},
		&AppTheme{},
		&CustomerAuthConfig{},
		&AppInputConfig{},
		// GitHub template customization models
		&GitHubRepoConfig{},
		&TemplateOverride{},
		&AssetOverride{},
		// Customer account models
		&CustomerAccount{},
		&CustomerAccountMember{},
		&CustomerAccountInvite{},
	)
	if err != nil {
		return nil, err
	}

	// Run workspace to org data migration (idempotent - safe to run multiple times)
	if err := runWorkspaceToOrgMigration(db); err != nil {
		return nil, fmt.Errorf("workspace to org migration failed: %w", err)
	}

	// Run subdomain migration (idempotent - populates subdomain on orgs from org names)
	if err := runOrgSubdomainMigration(db); err != nil {
		return nil, fmt.Errorf("org subdomain migration failed: %w", err)
	}

	// Run logo fields migration (idempotent - renames logo_base64 to logo_light_base64, adds logo_dark_base64)
	if err := runLogoFieldsMigration(db); err != nil {
		return nil, fmt.Errorf("logo fields migration failed: %w", err)
	}

	// Run workspace_id nullable migration (make workspace_id nullable since we're phasing it out)
	if err := runWorkspaceIDNullableMigration(db); err != nil {
		return nil, fmt.Errorf("workspace_id nullable migration failed: %w", err)
	}

	// Run install link name migration as a safety net (populates name field for any links missed)
	if err := ensureInstallLinkNamesPopulated(db); err != nil {
		return nil, fmt.Errorf("install link name migration failed: %w", err)
	}

	// Drop health check tables/columns (health check feature removed)
	if err := runDropAppHealthCheckConfigsMigration(db); err != nil {
		return nil, fmt.Errorf("drop app_health_check_configs migration failed: %w", err)
	}
	if err := runDropHealthCheckActionIDsMigration(db); err != nil {
		return nil, fmt.Errorf("drop health_check_action_ids migration failed: %w", err)
	}

	// Default existing installs to visibility='account'
	if err := runInstallVisibilityMigration(db); err != nil {
		return nil, fmt.Errorf("install visibility migration failed: %w", err)
	}

	// Add unique index on customer_account_members(user_id, org_id) to enforce one account per customer per org
	if err := runCustomerAccountMemberUniqueIndexMigration(db); err != nil {
		return nil, fmt.Errorf("customer account member unique index migration failed: %w", err)
	}

	return db, nil
}

// runInstallVisibilityMigration defaults existing installs to visibility='account'.
// This is idempotent - safe to run multiple times.
func runInstallVisibilityMigration(db *gorm.DB) error {
	var columnExists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'installs' AND column_name = 'visibility'
		)
	`).Scan(&columnExists).Error; err != nil {
		return fmt.Errorf("failed to check for visibility column: %w", err)
	}
	if !columnExists {
		return nil
	}

	// Update any installs with empty or null visibility to 'account'
	if err := db.Exec(`
		UPDATE installs SET visibility = 'account'
		WHERE (visibility IS NULL OR visibility = '') AND deleted_at IS NULL
	`).Error; err != nil {
		return fmt.Errorf("failed to set default visibility: %w", err)
	}

	// Add composite index on (customer_account_id, visibility) for install queries
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_installs_account_visibility ON installs (customer_account_id, visibility)`)

	return nil
}

// runCustomerAccountMemberUniqueIndexMigration drops the old unique index that enforced
// one account per customer per org. Customers can now belong to multiple accounts.
// This is idempotent - safe to run multiple times.
func runCustomerAccountMemberUniqueIndexMigration(db *gorm.DB) error {
	db.Exec(`DROP INDEX IF EXISTS idx_customer_account_members_user_org`)
	return nil
}

// runDropAppHealthCheckConfigsMigration drops the app_health_check_configs table if it exists.
// Idempotent - safe to run multiple times.
func runDropAppHealthCheckConfigsMigration(db *gorm.DB) error {
	if db.Migrator().HasTable("app_health_check_configs") {
		return db.Exec("DROP TABLE app_health_check_configs").Error
	}
	return nil
}

// runDropHealthCheckActionIDsMigration drops the health_check_action_ids column from install_links if it exists.
// Idempotent - safe to run multiple times.
func runDropHealthCheckActionIDsMigration(db *gorm.DB) error {
	if db.Migrator().HasColumn(&InstallLink{}, "health_check_action_ids") {
		return db.Exec("ALTER TABLE install_links DROP COLUMN health_check_action_ids").Error
	}
	return nil
}

// Ping checks the database connection health
func Ping(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// runWorkspaceToOrgMigration migrates data from the old workspace-based schema to the new org-based schema.
// This is idempotent - safe to run multiple times.
func runWorkspaceToOrgMigration(db *gorm.DB) error {
	// Check if old workspaces table exists
	var tableExists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'workspaces'
		)
	`).Scan(&tableExists).Error
	if err != nil {
		return fmt.Errorf("failed to check for workspaces table: %w", err)
	}

	// If workspaces table doesn't exist, no migration needed
	if !tableExists {
		return nil
	}

	// Check if migration has already been run by checking if org_members table has data
	var orgMemberCount int64
	if err := db.Model(&OrgMember{}).Count(&orgMemberCount).Error; err != nil {
		return fmt.Errorf("failed to count org members: %w", err)
	}

	// If org_members already has data, assume migration is complete
	if orgMemberCount > 0 {
		return nil
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		// Step 1: Migrate workspace_members to org_members
		// This maps workspace_id -> the corresponding nuon_org's ID
		if err := tx.Exec(`
			INSERT INTO org_members (id, org_id, user_id, invited_by, status, invited_at, joined_at, created_at, updated_at)
			SELECT
				wm.id,
				no.id as org_id,
				wm.user_id,
				wm.invited_by,
				wm.status,
				wm.invited_at,
				wm.joined_at,
				wm.created_at,
				wm.updated_at
			FROM workspace_members wm
			INNER JOIN nuon_orgs no ON no.workspace_id = wm.workspace_id
			WHERE wm.deleted_at IS NULL AND no.deleted_at IS NULL
			ON CONFLICT (id) DO NOTHING
		`).Error; err != nil {
			return fmt.Errorf("failed to migrate workspace_members to org_members: %w", err)
		}

		// Step 2: Migrate workspace_invitations to org_invitations
		if err := tx.Exec(`
			INSERT INTO org_invitations (id, org_id, email, invited_by, token, expires_at, accepted_at, used_count, max_uses, created_at, updated_at)
			SELECT
				wi.id,
				no.id as org_id,
				wi.email,
				wi.invited_by,
				wi.token,
				wi.expires_at,
				wi.accepted_at,
				wi.used_count,
				wi.max_uses,
				wi.created_at,
				wi.updated_at
			FROM workspace_invitations wi
			INNER JOIN nuon_orgs no ON no.workspace_id = wi.workspace_id
			WHERE wi.deleted_at IS NULL AND no.deleted_at IS NULL
			ON CONFLICT (id) DO NOTHING
		`).Error; err != nil {
			return fmt.Errorf("failed to migrate workspace_invitations to org_invitations: %w", err)
		}

		// Step 3: Copy subdomain from workspaces to nuon_orgs
		if err := tx.Exec(`
			UPDATE nuon_orgs
			SET subdomain = ws.subdomain
			FROM workspaces ws
			WHERE nuon_orgs.workspace_id = ws.id
			AND ws.subdomain IS NOT NULL
			AND ws.subdomain != ''
			AND (nuon_orgs.subdomain IS NULL OR nuon_orgs.subdomain = '')
		`).Error; err != nil {
			return fmt.Errorf("failed to copy subdomain to nuon_orgs: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Step 4: Update tables that referenced workspace_id to use the nuon_org's ID directly
	// Run these OUTSIDE the transaction since some tables may not exist yet.
	// Each update runs in its own implicit transaction.

	// Helper to safely run an optional migration
	safeExec := func(tableName, query string) {
		// First check if table exists
		var exists bool
		db.Raw(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public'
				AND table_name = ?
			)
		`, tableName).Scan(&exists)

		if !exists {
			return // Table doesn't exist, skip
		}

		// Check if workspace_id column exists
		var hasWorkspaceCol bool
		db.Raw(`
			SELECT EXISTS (
				SELECT FROM information_schema.columns
				WHERE table_schema = 'public'
				AND table_name = ?
				AND column_name = 'workspace_id'
			)
		`, tableName).Scan(&hasWorkspaceCol)

		if !hasWorkspaceCol {
			return // workspace_id column doesn't exist, skip
		}

		// Run the migration
		db.Exec(query)
	}

	// Update app_themes
	safeExec("app_themes", `
		UPDATE app_themes
		SET org_id = no.id
		FROM nuon_orgs no
		WHERE app_themes.workspace_id = no.workspace_id
		AND no.deleted_at IS NULL
		AND (app_themes.org_id IS NULL OR app_themes.org_id = '')
	`)

	// Update customer_auth_configs
	safeExec("customer_auth_configs", `
		UPDATE customer_auth_configs
		SET org_id = no.id
		FROM nuon_orgs no
		WHERE customer_auth_configs.workspace_id = no.workspace_id
		AND no.deleted_at IS NULL
		AND (customer_auth_configs.org_id IS NULL OR customer_auth_configs.org_id = '')
	`)

	// Update github_repo_configs
	safeExec("github_repo_configs", `
		UPDATE github_repo_configs
		SET org_id = no.id
		FROM nuon_orgs no
		WHERE github_repo_configs.workspace_id = no.workspace_id
		AND no.deleted_at IS NULL
		AND (github_repo_configs.org_id IS NULL OR github_repo_configs.org_id = '')
	`)

	// Update template_overrides
	safeExec("template_overrides", `
		UPDATE template_overrides
		SET org_id = no.id
		FROM nuon_orgs no
		WHERE template_overrides.workspace_id = no.workspace_id
		AND no.deleted_at IS NULL
		AND (template_overrides.org_id IS NULL OR template_overrides.org_id = '')
	`)

	// Update asset_overrides
	safeExec("asset_overrides", `
		UPDATE asset_overrides
		SET org_id = no.id
		FROM nuon_orgs no
		WHERE asset_overrides.workspace_id = no.workspace_id
		AND no.deleted_at IS NULL
		AND (asset_overrides.org_id IS NULL OR asset_overrides.org_id = '')
	`)

	// Update app_input_configs
	safeExec("app_input_configs", `
		UPDATE app_input_configs
		SET org_id = no.id
		FROM nuon_orgs no
		WHERE app_input_configs.workspace_id = no.workspace_id
		AND no.deleted_at IS NULL
		AND (app_input_configs.org_id IS NULL OR app_input_configs.org_id = '')
	`)

	// Update install_links
	safeExec("install_links", `
		UPDATE install_links
		SET org_id = no.id
		FROM nuon_orgs no
		WHERE install_links.workspace_id = no.workspace_id
		AND no.deleted_at IS NULL
		AND (install_links.org_id IS NULL OR install_links.org_id = '')
	`)

	// Update installs
	safeExec("installs", `
		UPDATE installs
		SET org_id = no.id
		FROM nuon_orgs no
		WHERE installs.workspace_id = no.workspace_id
		AND no.deleted_at IS NULL
		AND (installs.org_id IS NULL OR installs.org_id = '')
	`)

	return nil
}

// isColumnOrTableNotExistsError checks if the error is about a column or table not existing
// This allows the migration to gracefully skip tables/columns that don't exist yet
func isColumnNotExistsError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Check for common "does not exist" patterns (column, relation/table)
	return strings.Contains(errStr, "does not exist") ||
		strings.Contains(errStr, "42P01") || // PostgreSQL: undefined_table
		strings.Contains(errStr, "42703") // PostgreSQL: undefined_column
}

// runOrgSubdomainMigration populates the subdomain field for existing orgs
// that don't have one. Uses the org name as the base for subdomain.
// This is idempotent - safe to run multiple times.
func runOrgSubdomainMigration(db *gorm.DB) error {
	// Check if migration is needed (any org without subdomain)
	var count int64
	if err := db.Model(&NuonOrg{}).Where("(subdomain IS NULL OR subdomain = '') AND deleted_at IS NULL").Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check for orgs without subdomain: %w", err)
	}

	// No orgs need migration
	if count == 0 {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		// Get all orgs without subdomains
		var orgs []NuonOrg
		if err := tx.Where("(subdomain IS NULL OR subdomain = '') AND deleted_at IS NULL").Find(&orgs).Error; err != nil {
			return fmt.Errorf("failed to fetch orgs: %w", err)
		}

		// Track used subdomains to avoid collisions
		usedSubdomains := make(map[string]bool)

		// First, get all existing subdomains
		var existingSubdomains []string
		if err := tx.Model(&NuonOrg{}).Where("subdomain IS NOT NULL AND subdomain != '' AND deleted_at IS NULL").Pluck("subdomain", &existingSubdomains).Error; err != nil {
			return fmt.Errorf("failed to fetch existing subdomains: %w", err)
		}
		for _, s := range existingSubdomains {
			usedSubdomains[s] = true
		}

		for _, org := range orgs {
			// Use org name as base for subdomain
			baseName := org.Name
			if baseName == "" {
				baseName = "org"
			}

			// Normalize the subdomain
			subdomain := NormalizeSubdomain(baseName)

			// Handle collisions by appending suffix
			finalSubdomain := subdomain
			suffix := 1
			for usedSubdomains[finalSubdomain] || ReservedSubdomains[finalSubdomain] {
				finalSubdomain = fmt.Sprintf("%s-%d", subdomain, suffix)
				// Ensure we don't exceed max length
				if len(finalSubdomain) > 63 {
					maxBaseLen := 63 - len(fmt.Sprintf("-%d", suffix))
					finalSubdomain = fmt.Sprintf("%s-%d", subdomain[:maxBaseLen], suffix)
				}
				suffix++
			}

			usedSubdomains[finalSubdomain] = true

			if err := tx.Model(&org).Update("subdomain", finalSubdomain).Error; err != nil {
				return fmt.Errorf("failed to update subdomain for org %s: %w", org.ID, err)
			}
		}

		return nil
	})
}

// runLogoFieldsMigration renames logo_base64 to logo_light_base64 and adds logo_dark_base64 column.
// This is idempotent - safe to run multiple times.
func runLogoFieldsMigration(db *gorm.DB) error {
	// Check if migration has already been run by checking for logo_light_base64 column
	var columnExists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'app_themes' AND column_name = 'logo_light_base64'
		)
	`).Scan(&columnExists).Error
	if err != nil {
		return fmt.Errorf("failed to check for logo_light_base64 column: %w", err)
	}

	// If logo_light_base64 already exists, migration is complete
	if columnExists {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		// Check if old logo_base64 column exists
		var oldColumnExists bool
		err := tx.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'app_themes' AND column_name = 'logo_base64'
			)
		`).Scan(&oldColumnExists).Error
		if err != nil {
			return fmt.Errorf("failed to check for logo_base64 column: %w", err)
		}

		// If old column exists, rename it to logo_light_base64
		if oldColumnExists {
			if err := tx.Exec(`ALTER TABLE app_themes RENAME COLUMN logo_base64 TO logo_light_base64`).Error; err != nil {
				return fmt.Errorf("failed to rename logo_base64 column: %w", err)
			}
		} else {
			// If old column doesn't exist, create logo_light_base64 directly
			if err := tx.Exec(`ALTER TABLE app_themes ADD COLUMN IF NOT EXISTS logo_light_base64 TEXT`).Error; err != nil {
				return fmt.Errorf("failed to add logo_light_base64 column: %w", err)
			}
		}

		// Add logo_dark_base64 column
		if err := tx.Exec(`ALTER TABLE app_themes ADD COLUMN IF NOT EXISTS logo_dark_base64 TEXT`).Error; err != nil {
			return fmt.Errorf("failed to add logo_dark_base64 column: %w", err)
		}

		return nil
	})
}

// runWorkspaceIDNullableMigration makes the workspace_id column nullable on nuon_orgs.
// This is needed because we're phasing out the workspace abstraction and consolidating
// everything into NuonOrg. The column needs to be nullable to prevent constraint violations
// when GORM cascades updates to NuonOrg records (since the model no longer has WorkspaceID).
// This is idempotent - safe to run multiple times.
func runWorkspaceIDNullableMigration(db *gorm.DB) error {
	// Check if workspace_id column exists on nuon_orgs
	var columnExists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'nuon_orgs' AND column_name = 'workspace_id'
		)
	`).Scan(&columnExists).Error
	if err != nil {
		return fmt.Errorf("failed to check for workspace_id column: %w", err)
	}

	// If workspace_id column doesn't exist, nothing to do
	if !columnExists {
		return nil
	}

	// Check if workspace_id column is NOT NULL
	var isNotNull bool
	err = db.Raw(`
		SELECT is_nullable = 'NO'
		FROM information_schema.columns
		WHERE table_name = 'nuon_orgs' AND column_name = 'workspace_id'
	`).Scan(&isNotNull).Error
	if err != nil {
		return fmt.Errorf("failed to check workspace_id nullable status: %w", err)
	}

	// If column is already nullable, nothing to do
	if !isNotNull {
		return nil
	}

	// Make workspace_id nullable
	if err := db.Exec(`ALTER TABLE nuon_orgs ALTER COLUMN workspace_id DROP NOT NULL`).Error; err != nil {
		return fmt.Errorf("failed to make workspace_id nullable: %w", err)
	}

	return nil
}

// prepareInstallLinkNameColumn adds the name column as nullable and populates existing rows
// BEFORE AutoMigrate runs. This prevents the error:
// "column 'name' of relation 'install_links' contains null values"
// which occurs when AutoMigrate tries to add `name text NOT NULL` to a table with existing rows.
// This is idempotent - safe to run multiple times.
func prepareInstallLinkNameColumn(db *gorm.DB) error {
	// Check if install_links table exists
	var tableExists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'install_links'
		)
	`).Scan(&tableExists).Error
	if err != nil {
		return fmt.Errorf("failed to check for install_links table: %w", err)
	}

	// If table doesn't exist, nothing to do - AutoMigrate will create it with the column
	if !tableExists {
		return nil
	}

	// Check if name column already exists
	var columnExists bool
	err = db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'install_links' AND column_name = 'name'
		)
	`).Scan(&columnExists).Error
	if err != nil {
		return fmt.Errorf("failed to check for name column: %w", err)
	}

	// If column doesn't exist, add it as NULLABLE first
	if !columnExists {
		if err := db.Exec(`ALTER TABLE install_links ADD COLUMN name text`).Error; err != nil {
			return fmt.Errorf("failed to add name column: %w", err)
		}
	}

	// Populate any rows that have NULL or empty names
	// Pattern: {app_name}-install-{created_at_unix}
	if err := db.Exec(`
		UPDATE install_links
		SET name = CONCAT(app_name, '-install-', EXTRACT(EPOCH FROM created_at)::bigint)
		WHERE (name IS NULL OR name = '') AND deleted_at IS NULL
	`).Error; err != nil {
		return fmt.Errorf("failed to populate install link names: %w", err)
	}

	return nil
}

// runInstallLinkIDNullableMigration makes the install_link_id column nullable on installs.
// This is needed for published-app installs that don't have an install link.
// This is idempotent - safe to run multiple times.
func runInstallLinkIDNullableMigration(db *gorm.DB) error {
	// Check if installs table exists
	var tableExists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'installs'
		)
	`).Scan(&tableExists).Error
	if err != nil {
		return fmt.Errorf("failed to check for installs table: %w", err)
	}

	// If installs table doesn't exist, nothing to do - AutoMigrate will create it
	if !tableExists {
		return nil
	}

	// Check if install_link_id column is NOT NULL
	var isNotNull bool
	err = db.Raw(`
		SELECT is_nullable = 'NO'
		FROM information_schema.columns
		WHERE table_name = 'installs' AND column_name = 'install_link_id'
	`).Scan(&isNotNull).Error
	if err != nil {
		return fmt.Errorf("failed to check install_link_id nullable status: %w", err)
	}

	// If column is already nullable, nothing to do
	if !isNotNull {
		return nil
	}

	// Make install_link_id nullable
	if err := db.Exec(`ALTER TABLE installs ALTER COLUMN install_link_id DROP NOT NULL`).Error; err != nil {
		return fmt.Errorf("failed to make install_link_id nullable: %w", err)
	}

	return nil
}

// runCreatedByVendorIDNullableMigration makes the created_by_vendor_id column nullable on installs.
// This is needed for published-app installs that don't have a vendor.
// This is idempotent - safe to run multiple times.
func runCreatedByVendorIDNullableMigration(db *gorm.DB) error {
	var tableExists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = 'installs'
		)
	`).Scan(&tableExists).Error; err != nil {
		return fmt.Errorf("failed to check for installs table: %w", err)
	}
	if !tableExists {
		return nil
	}

	var isNotNull bool
	if err := db.Raw(`
		SELECT is_nullable = 'NO'
		FROM information_schema.columns
		WHERE table_name = 'installs' AND column_name = 'created_by_vendor_id'
	`).Scan(&isNotNull).Error; err != nil {
		return fmt.Errorf("failed to check created_by_vendor_id nullable status: %w", err)
	}
	if !isNotNull {
		return nil
	}

	if err := db.Exec(`ALTER TABLE installs ALTER COLUMN created_by_vendor_id DROP NOT NULL`).Error; err != nil {
		return fmt.Errorf("failed to make created_by_vendor_id nullable: %w", err)
	}

	return nil
}

// ensureInstallLinkNamesPopulated is a safety net that runs AFTER AutoMigrate.
// It populates the name field for any install links that might have been missed.
// Uses the pattern: {app_name}-install-{created_at_unix}
// This is idempotent - safe to run multiple times.
func ensureInstallLinkNamesPopulated(db *gorm.DB) error {
	// Check if name column exists on install_links
	var columnExists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'install_links' AND column_name = 'name'
		)
	`).Scan(&columnExists).Error
	if err != nil {
		return fmt.Errorf("failed to check for name column: %w", err)
	}

	// If name column doesn't exist yet, GORM AutoMigrate will create it
	// We just need to populate existing rows after that happens
	if !columnExists {
		return nil
	}

	// Check if migration is needed (any install links without a name)
	var count int64
	if err := db.Model(&InstallLink{}).Where("(name IS NULL OR name = '') AND deleted_at IS NULL").Count(&count).Error; err != nil {
		// Column might not exist yet if AutoMigrate hasn't run
		if strings.Contains(err.Error(), "does not exist") {
			return nil
		}
		return fmt.Errorf("failed to check for install links without name: %w", err)
	}

	// No links need migration
	if count == 0 {
		return nil
	}

	// Update all install links without names using the pattern: {app_name}-install-{created_at_unix}
	// This is safe because we're generating unique names based on timestamp
	if err := db.Exec(`
		UPDATE install_links
		SET name = CONCAT(app_name, '-install-', EXTRACT(EPOCH FROM created_at)::bigint)
		WHERE (name IS NULL OR name = '') AND deleted_at IS NULL
	`).Error; err != nil {
		return fmt.Errorf("failed to populate install link names: %w", err)
	}

	return nil
}

// runInviteEmailMigration migrates customer_account_invites from the old token-based
// schema to the new email-based schema. Deletes all existing rows (old token-based
// invites are no longer usable) and drops obsolete columns so AutoMigrate can safely
// add the email NOT NULL column.
// This is idempotent - safe to run multiple times.
func runInviteEmailMigration(db *gorm.DB) error {
	if !db.Migrator().HasTable("customer_account_invites") {
		return nil
	}
	if db.Migrator().HasColumn(&CustomerAccountInvite{}, "token") {
		db.Exec("DELETE FROM customer_account_invites")
		db.Migrator().DropColumn(&CustomerAccountInvite{}, "token")
		db.Migrator().DropColumn(&CustomerAccountInvite{}, "expires_at")
	}
	return nil
}
