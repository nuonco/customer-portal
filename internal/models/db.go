package models

import (
	"context"
	"fmt"
	"os"
	"time"

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

	// Auto-migrate all models (including new workspace models)
	err = db.AutoMigrate(
		&User{},
		&Workspace{},
		&WorkspaceMember{},
		&WorkspaceInvitation{},
		&NuonOrg{},
		&InstallLink{},
		&Install{},
		&AppTheme{},
		&AppHealthCheckConfig{},
		&CustomerAuthConfig{},
		&AppInputConfig{},
		// GitHub template customization models
		&GitHubRepoConfig{},
		&TemplateOverride{},
		&AssetOverride{},
	)
	if err != nil {
		return nil, err
	}

	// Run workspace data migration (idempotent - safe to run multiple times)
	if err := runWorkspaceMigration(db); err != nil {
		return nil, fmt.Errorf("workspace migration failed: %w", err)
	}

	// Run one-to-one workspace-org migration (idempotent)
	if err := runOneToOneMigration(db); err != nil {
		return nil, fmt.Errorf("one-to-one migration failed: %w", err)
	}

	// Run subdomain migration (idempotent - populates subdomain from org names)
	if err := runSubdomainMigration(db); err != nil {
		return nil, fmt.Errorf("subdomain migration failed: %w", err)
	}

	// Run logo fields migration (idempotent - renames logo_base64 to logo_light_base64, adds logo_dark_base64)
	if err := runLogoFieldsMigration(db); err != nil {
		return nil, fmt.Errorf("logo fields migration failed: %w", err)
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

// runWorkspaceMigration performs the data migration to workspace-scoped resources.
// This is idempotent - safe to run multiple times.
func runWorkspaceMigration(db *gorm.DB) error {
	// Check if migration has already been run by counting workspaces
	var workspaceCount int64
	if err := db.Model(&Workspace{}).Count(&workspaceCount).Error; err != nil {
		return fmt.Errorf("failed to count workspaces: %w", err)
	}

	// If workspaces already exist, assume migration is complete
	if workspaceCount > 0 {
		return nil
	}

	// Begin transaction for data migration
	return db.Transaction(func(tx *gorm.DB) error {
		// Step 1: Create personal workspaces for all vendor users
		var vendorUsers []User
		if err := tx.Where("role = ? AND deleted_at IS NULL", RoleVendor).Find(&vendorUsers).Error; err != nil {
			return fmt.Errorf("failed to fetch vendor users: %w", err)
		}

		workspaceMap := make(map[string]string) // userID -> workspaceID

		for _, user := range vendorUsers {
			workspaceName := user.Name
			if workspaceName == "" {
				workspaceName = user.Email
			}
			workspaceName = workspaceName + "'s Personal Workspace"

			workspace := Workspace{
				Name:       workspaceName,
				IsPersonal: true,
			}
			if err := tx.Create(&workspace).Error; err != nil {
				return fmt.Errorf("failed to create workspace for user %s: %w", user.ID, err)
			}

			workspaceMap[user.ID] = workspace.ID

			// Add user as active member
			now := time.Now()
			member := WorkspaceMember{
				WorkspaceID: workspace.ID,
				UserID:      user.ID,
				Status:      MemberStatusActive,
				JoinedAt:    &now,
			}
			if err := tx.Create(&member).Error; err != nil {
				return fmt.Errorf("failed to create workspace member for user %s: %w", user.ID, err)
			}
		}

		// Step 2: Migrate nuon_orgs to personal workspaces
		if err := tx.Exec(`
			UPDATE nuon_orgs
			SET workspace_id = (
				SELECT w.id FROM workspaces w
				INNER JOIN workspace_members wm ON wm.workspace_id = w.id
				WHERE wm.user_id = nuon_orgs.user_id AND w.is_personal = true
				LIMIT 1
			)
			WHERE deleted_at IS NULL AND workspace_id IS NULL
		`).Error; err != nil {
			return fmt.Errorf("failed to migrate nuon_orgs: %w", err)
		}

		// Step 3: Migrate install_links to personal workspaces
		if err := tx.Exec(`
			UPDATE install_links
			SET workspace_id = (
				SELECT w.id FROM workspaces w
				INNER JOIN workspace_members wm ON wm.workspace_id = w.id
				WHERE wm.user_id = install_links.user_id AND w.is_personal = true
				LIMIT 1
			)
			WHERE deleted_at IS NULL AND workspace_id IS NULL
		`).Error; err != nil {
			return fmt.Errorf("failed to migrate install_links: %w", err)
		}

		// Step 4: Migrate app_input_configs via nuon_org lookup
		if err := tx.Exec(`
			UPDATE app_input_configs
			SET workspace_id = (
				SELECT workspace_id FROM nuon_orgs
				WHERE nuon_orgs.id = app_input_configs.org_id
				LIMIT 1
			)
			WHERE deleted_at IS NULL AND workspace_id IS NULL
		`).Error; err != nil {
			return fmt.Errorf("failed to migrate app_input_configs: %w", err)
		}

		// Step 5: Migrate installs to vendor workspaces (based on who created them)
		if err := tx.Exec(`
			UPDATE installs
			SET workspace_id = (
				SELECT w.id FROM workspaces w
				INNER JOIN workspace_members wm ON wm.workspace_id = w.id
				WHERE wm.user_id = installs.created_by_vendor_id AND w.is_personal = true
				LIMIT 1
			)
			WHERE deleted_at IS NULL AND workspace_id IS NULL
		`).Error; err != nil {
			return fmt.Errorf("failed to migrate installs: %w", err)
		}

		// Step 6: Get existing theme and auth config (if any) for duplication
		var existingTheme AppTheme
		hasTheme := tx.First(&existingTheme).Error == nil

		var existingAuthConfig CustomerAuthConfig
		hasAuthConfig := tx.First(&existingAuthConfig).Error == nil

		// Step 7: Create theme and auth config for each workspace
		var workspaces []Workspace
		if err := tx.Find(&workspaces).Error; err != nil {
			return fmt.Errorf("failed to fetch workspaces: %w", err)
		}

		for _, workspace := range workspaces {
			// Create theme for this workspace
			theme := AppTheme{
				WorkspaceID:    workspace.ID,
				PrimaryColor:   DefaultPrimaryColor,
				SecondaryColor: DefaultPrimaryColor,
				BorderRadius:   DefaultBorderRadius,
				SpacingDensity: DefaultSpacingDensity,
			}
			if hasTheme {
				// Copy settings from existing theme
				theme.PrimaryColor = existingTheme.PrimaryColor
				theme.SecondaryColor = existingTheme.SecondaryColor
				theme.LogoLightBase64 = existingTheme.LogoLightBase64
				theme.LogoDarkBase64 = existingTheme.LogoDarkBase64
				theme.SupportContact = existingTheme.SupportContact
				theme.HeadingFont = existingTheme.HeadingFont
				theme.BodyFont = existingTheme.BodyFont
				theme.HeadingFontBase64 = existingTheme.HeadingFontBase64
				theme.BodyFontBase64 = existingTheme.BodyFontBase64
				theme.BorderRadius = existingTheme.BorderRadius
				theme.SpacingDensity = existingTheme.SpacingDensity
				theme.LoginTitle = existingTheme.LoginTitle
				theme.LoginSubtitle = existingTheme.LoginSubtitle
			}
			if err := tx.Create(&theme).Error; err != nil {
				return fmt.Errorf("failed to create theme for workspace %s: %w", workspace.ID, err)
			}

			// Create auth config for this workspace
			authConfig := CustomerAuthConfig{
				WorkspaceID: workspace.ID,
				Enabled:     false,
				Scopes:      DefaultScopes,
			}
			if hasAuthConfig {
				// Copy settings from existing config
				authConfig.Enabled = existingAuthConfig.Enabled
				authConfig.ProviderName = existingAuthConfig.ProviderName
				authConfig.ClientID = existingAuthConfig.ClientID
				authConfig.ClientSecret = existingAuthConfig.ClientSecret
				authConfig.IssuerURL = existingAuthConfig.IssuerURL
				authConfig.Scopes = existingAuthConfig.Scopes
			}
			if err := tx.Create(&authConfig).Error; err != nil {
				return fmt.Errorf("failed to create auth config for workspace %s: %w", workspace.ID, err)
			}
		}

		// Step 8: Delete old global theme and auth config (if they exist and have no workspace_id)
		if hasTheme {
			if err := tx.Where("workspace_id IS NULL OR workspace_id = ''").Delete(&AppTheme{}).Error; err != nil {
				return fmt.Errorf("failed to delete old theme: %w", err)
			}
		}
		if hasAuthConfig {
			if err := tx.Where("workspace_id IS NULL OR workspace_id = ''").Delete(&CustomerAuthConfig{}).Error; err != nil {
				return fmt.Errorf("failed to delete old auth config: %w", err)
			}
		}

		// Step 9: Verify migration - check for orphaned records
		var orphanedCounts struct {
			Orgs     int64
			Links    int64
			Configs  int64
			Installs int64
		}

		tx.Model(&NuonOrg{}).Where("workspace_id IS NULL AND deleted_at IS NULL").Count(&orphanedCounts.Orgs)
		tx.Model(&InstallLink{}).Where("workspace_id IS NULL AND deleted_at IS NULL").Count(&orphanedCounts.Links)
		tx.Model(&AppInputConfig{}).Where("workspace_id IS NULL AND deleted_at IS NULL").Count(&orphanedCounts.Configs)
		tx.Model(&Install{}).Where("workspace_id IS NULL AND deleted_at IS NULL").Count(&orphanedCounts.Installs)

		if orphanedCounts.Orgs > 0 || orphanedCounts.Links > 0 || orphanedCounts.Configs > 0 || orphanedCounts.Installs > 0 {
			return fmt.Errorf("migration incomplete: found orphaned records (orgs:%d links:%d configs:%d installs:%d)",
				orphanedCounts.Orgs, orphanedCounts.Links, orphanedCounts.Configs, orphanedCounts.Installs)
		}

		return nil
	})
}

// runOneToOneMigration enforces one-to-one relationship between workspaces and orgs.
// This is idempotent - safe to run multiple times.
func runOneToOneMigration(db *gorm.DB) error {
	// Check if migration has already been run by checking for the unique index
	var indexExists int
	db.Raw(`
		SELECT 1 FROM pg_indexes
		WHERE indexname = 'idx_unique_workspace_org'
	`).Scan(&indexExists)

	if indexExists == 1 {
		// Migration already complete
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		// Step 1: For workspaces with multiple orgs, keep only the oldest one (by created_at)
		// Soft-delete all other orgs for that workspace
		if err := tx.Exec(`
			UPDATE nuon_orgs
			SET deleted_at = NOW()
			WHERE deleted_at IS NULL
			AND id NOT IN (
				SELECT DISTINCT ON (workspace_id) id
				FROM nuon_orgs
				WHERE deleted_at IS NULL AND workspace_id IS NOT NULL AND workspace_id != ''
				ORDER BY workspace_id, created_at ASC
			)
			AND workspace_id IS NOT NULL
			AND workspace_id != ''
		`).Error; err != nil {
			return fmt.Errorf("failed to soft-delete extra orgs: %w", err)
		}

		// Step 2: Soft-delete workspaces that have no connected orgs
		if err := tx.Exec(`
			UPDATE workspaces
			SET deleted_at = NOW()
			WHERE deleted_at IS NULL
			AND id NOT IN (
				SELECT DISTINCT workspace_id
				FROM nuon_orgs
				WHERE deleted_at IS NULL AND workspace_id IS NOT NULL AND workspace_id != ''
			)
		`).Error; err != nil {
			return fmt.Errorf("failed to soft-delete orphaned workspaces: %w", err)
		}

		// Step 3: Add unique partial index on workspace_id (one org per workspace)
		if err := tx.Exec(`
			CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_workspace_org
			ON nuon_orgs (workspace_id)
			WHERE deleted_at IS NULL
		`).Error; err != nil {
			return fmt.Errorf("failed to create workspace unique index: %w", err)
		}

		// Step 4: Add unique partial index on org_id (one workspace per Nuon org)
		if err := tx.Exec(`
			CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_nuon_org
			ON nuon_orgs (org_id)
			WHERE deleted_at IS NULL
		`).Error; err != nil {
			return fmt.Errorf("failed to create nuon_org unique index: %w", err)
		}

		return nil
	})
}

// runSubdomainMigration populates the subdomain field for existing workspaces
// that don't have one. Uses the org name (via NuonOrg) as the base for subdomain.
// This is idempotent - safe to run multiple times.
func runSubdomainMigration(db *gorm.DB) error {
	// Check if migration is needed (any workspace without subdomain)
	var count int64
	if err := db.Model(&Workspace{}).Where("(subdomain IS NULL OR subdomain = '') AND deleted_at IS NULL").Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check for workspaces without subdomain: %w", err)
	}

	// No workspaces need migration
	if count == 0 {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		// Get all workspaces with their org names
		var workspaces []Workspace
		if err := tx.Preload("NuonOrg").Where("(subdomain IS NULL OR subdomain = '') AND deleted_at IS NULL").Find(&workspaces).Error; err != nil {
			return fmt.Errorf("failed to fetch workspaces: %w", err)
		}

		// Track used subdomains to avoid collisions
		usedSubdomains := make(map[string]bool)

		// First, get all existing subdomains
		var existingSubdomains []string
		if err := tx.Model(&Workspace{}).Where("subdomain IS NOT NULL AND subdomain != '' AND deleted_at IS NULL").Pluck("subdomain", &existingSubdomains).Error; err != nil {
			return fmt.Errorf("failed to fetch existing subdomains: %w", err)
		}
		for _, s := range existingSubdomains {
			usedSubdomains[s] = true
		}

		for _, ws := range workspaces {
			// Get the base name from org, fall back to workspace name
			baseName := ws.Name
			if ws.NuonOrg != nil && ws.NuonOrg.Name != "" {
				baseName = ws.NuonOrg.Name
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

			if err := tx.Model(&ws).Update("subdomain", finalSubdomain).Error; err != nil {
				return fmt.Errorf("failed to update subdomain for workspace %s: %w", ws.ID, err)
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
