package models

import (
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

// installNameRegex matches valid install names:
// - Letters (uppercase/lowercase)
// - Numbers
// - Spaces
// - Dashes
// - Underscores
var installNameRegex = regexp.MustCompile(`^[a-zA-Z0-9 _-]+$`)

// NormalizeInstallName trims leading/trailing whitespace from an install name.
// Unlike subdomains, install names preserve case and internal spaces.
func NormalizeInstallName(name string) string {
	return strings.TrimSpace(name)
}

// ValidateInstallName checks if an install name is valid.
// Valid names:
// - 1-255 characters
// - Non-empty after trimming
// - Only letters, numbers, spaces, dashes, and underscores
func ValidateInstallName(name string) error {
	if name == "" {
		return fmt.Errorf("install name cannot be empty")
	}

	if len(name) > 255 {
		return fmt.Errorf("install name cannot exceed 255 characters")
	}

	if !installNameRegex.MatchString(name) {
		return fmt.Errorf("install name can only contain letters, numbers, spaces, dashes, and underscores")
	}

	return nil
}

// IsInstallNameAvailableLocally checks if an install name is available in the local InstallLink table.
// It checks for unused install links with the same name for the same app (case-insensitive).
// The excludeLinkID parameter allows excluding a specific link (useful for updates).
func IsInstallNameAvailableLocally(db *gorm.DB, orgID, appID, name, excludeLinkID string) (bool, error) {
	var count int64

	// Build query for unused install links with same name (case-insensitive) for the same app
	query := db.Model(&InstallLink{}).
		Where("org_id = ?", orgID).
		Where("app_id = ?", appID).
		Where("LOWER(name) = LOWER(?)", name).
		Where("used = ?", false).
		Where("deleted_at IS NULL")

	// Exclude a specific link if provided (useful for updates)
	if excludeLinkID != "" {
		query = query.Where("id != ?", excludeLinkID)
	}

	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check install name availability: %w", err)
	}

	return count == 0, nil
}
