package models

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"
)

// ReservedSubdomains contains subdomains that cannot be used by orgs
var ReservedSubdomains = map[string]bool{
	"www":     true,
	"api":     true,
	"admin":   true,
	"app":     true,
	"portal":  true,
	"console": true,
	"help":    true,
	"support": true,
	"status":  true,
	"login":   true,
	"auth":    true,
	"static":  true,
	"assets":  true,
	"cdn":     true,
	"mail":    true,
	"smtp":    true,
	"ftp":     true,
}

// NormalizeSubdomain converts a name (typically org name) to a valid subdomain
// - Converts to lowercase
// - Replaces spaces and underscores with dashes
// - Removes all characters except lowercase letters, numbers, and dashes
// - Collapses multiple dashes into one
// - Trims leading/trailing dashes
// - Truncates to 63 characters (DNS label limit)
func NormalizeSubdomain(name string) string {
	// Normalize unicode (e.g., convert accented chars to base form)
	name = norm.NFKD.String(name)

	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace spaces and underscores with dashes
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	// Remove all characters except lowercase letters, numbers, and dashes
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	name = reg.ReplaceAllString(name, "")

	// Collapse multiple dashes into one
	reg = regexp.MustCompile(`-+`)
	name = reg.ReplaceAllString(name, "-")

	// Trim leading/trailing dashes
	name = strings.Trim(name, "-")

	// Truncate to 63 characters (DNS label limit)
	if len(name) > 63 {
		name = name[:63]
		// Ensure we don't end with a dash after truncation
		name = strings.TrimRight(name, "-")
	}

	// If empty after normalization, generate a fallback
	if name == "" {
		name = "org"
	}

	return name
}

// ValidateSubdomain checks if a subdomain is valid and not reserved
func ValidateSubdomain(subdomain string) error {
	if subdomain == "" {
		return fmt.Errorf("subdomain cannot be empty")
	}

	if len(subdomain) > 63 {
		return fmt.Errorf("subdomain cannot exceed 63 characters")
	}

	if len(subdomain) < 2 {
		return fmt.Errorf("subdomain must be at least 2 characters")
	}

	// Must start with a letter
	if !unicode.IsLetter(rune(subdomain[0])) {
		return fmt.Errorf("subdomain must start with a letter")
	}

	// Must end with a letter or number
	lastChar := rune(subdomain[len(subdomain)-1])
	if !unicode.IsLetter(lastChar) && !unicode.IsDigit(lastChar) {
		return fmt.Errorf("subdomain must end with a letter or number")
	}

	// Only lowercase letters, numbers, and dashes allowed
	for _, r := range subdomain {
		if !unicode.IsLower(r) && !unicode.IsDigit(r) && r != '-' {
			return fmt.Errorf("subdomain can only contain lowercase letters, numbers, and dashes")
		}
	}

	// Check reserved words
	if ReservedSubdomains[subdomain] {
		return fmt.Errorf("'%s' is a reserved subdomain", subdomain)
	}

	return nil
}

// IsSubdomainAvailable checks if a subdomain is available (not used by another org)
func IsSubdomainAvailable(db *gorm.DB, subdomain string, excludeOrgID string) (bool, error) {
	var count int64
	query := db.Model(&NuonOrg{}).Where("subdomain = ? AND deleted_at IS NULL", subdomain)
	if excludeOrgID != "" {
		query = query.Where("id != ?", excludeOrgID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

// GenerateUniqueSubdomain generates a unique subdomain from a base name
// If the normalized subdomain is taken, it appends -1, -2, etc. until finding an available one
func GenerateUniqueSubdomain(db *gorm.DB, baseName string, excludeOrgID string) (string, error) {
	subdomain := NormalizeSubdomain(baseName)

	// Check if the base subdomain is available and valid
	if err := ValidateSubdomain(subdomain); err == nil {
		available, err := IsSubdomainAvailable(db, subdomain, excludeOrgID)
		if err != nil {
			return "", err
		}
		if available {
			return subdomain, nil
		}
	}

	// Try with numeric suffixes
	for i := 1; i <= 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", subdomain, i)
		// Ensure we don't exceed max length
		if len(candidate) > 63 {
			// Truncate base subdomain to make room for suffix
			maxBaseLen := 63 - len(fmt.Sprintf("-%d", i))
			candidate = fmt.Sprintf("%s-%d", subdomain[:maxBaseLen], i)
		}

		if err := ValidateSubdomain(candidate); err != nil {
			continue
		}

		available, err := IsSubdomainAvailable(db, candidate, excludeOrgID)
		if err != nil {
			return "", err
		}
		if available {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not generate unique subdomain after 1000 attempts")
}
