package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

type InstallLink struct {
	ID                   string         `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	OrgID                string         `gorm:"column:org_id;not null;index" json:"org_id"` // FK to NuonOrg.ID (local database ID)
	UserID               string         `gorm:"not null" json:"user_id"`                    // Kept for audit trail (who created it)
	AppID                string         `gorm:"not null" json:"app_id"`
	AppName              string         `gorm:"not null" json:"app_name"`
	SHA                  string         `gorm:"uniqueIndex;not null" json:"sha"`
	Used                 bool           `gorm:"default:false" json:"used"`
	HealthCheckActionIDs string         `json:"health_check_action_ids"`        // Comma-separated action workflow IDs
	VendorInputs         string         `gorm:"type:text" json:"vendor_inputs"` // JSON-encoded map[string]string
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	User    User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	NuonOrg NuonOrg  `gorm:"foreignKey:OrgID;references:ID" json:"nuon_org,omitempty"`
	Install *Install `gorm:"foreignKey:InstallLinkID" json:"install,omitempty"`
}

func (il *InstallLink) BeforeCreate(tx *gorm.DB) error {
	if il.ID == "" {
		il.ID = shortid.NewInstallLinkID()
	}
	return nil
}

func (il *InstallLink) GetInstallURL(baseURL string) string {
	return baseURL + "/install-link?sha=" + il.SHA
}

// GetInstallURLWithSubdomain returns the install URL with org subdomain if configured
// Falls back to base URL if org has no subdomain configured
func (il *InstallLink) GetInstallURLWithSubdomain(baseURL, baseDomain string) string {
	// If org is preloaded and has a subdomain, construct subdomain URL
	if il.NuonOrg.Subdomain != "" {
		subdomainURL := constructSubdomainURL(baseURL, baseDomain, il.NuonOrg.Subdomain)
		return subdomainURL + "/install-link?sha=" + il.SHA
	}

	// Fallback to base URL (existing behavior)
	return il.GetInstallURL(baseURL)
}

// constructSubdomainURL builds a URL with subdomain inserted properly
// Handles both localhost (with port) and production domains
// Examples:
//
//	baseURL: "http://localhost:8080", baseDomain: "localhost:8080", subdomain: "acme"
//	-> "http://acme.localhost:8080"
//
//	baseURL: "https://portal.nuon.co", baseDomain: "portal.nuon.co", subdomain: "acme"
//	-> "https://acme.portal.nuon.co"
func constructSubdomainURL(baseURL, baseDomain, subdomain string) string {
	// Parse scheme from baseURL (http:// or https://)
	scheme := "https://"
	if strings.HasPrefix(baseURL, "http://") {
		scheme = "http://"
	}

	// Construct subdomain URL: scheme + subdomain + . + baseDomain
	return fmt.Sprintf("%s%s.%s", scheme, subdomain, baseDomain)
}

// GetHealthCheckActionIDs returns the health check action IDs as a slice
// These may be in "configId:workflowId" format or just "actionId" for legacy data
func (il *InstallLink) GetHealthCheckActionIDs() []string {
	if il.HealthCheckActionIDs == "" {
		return []string{}
	}
	return strings.Split(il.HealthCheckActionIDs, ",")
}

// SetHealthCheckActionIDs sets the health check action IDs from a slice
func (il *InstallLink) SetHealthCheckActionIDs(ids []string) {
	il.HealthCheckActionIDs = strings.Join(ids, ",")
}

// HealthCheckIDPair represents a parsed health check ID with both config and workflow IDs
type HealthCheckIDPair struct {
	ConfigID   string // Used for triggering actions
	WorkflowID string // Used for status checking
}

// ParseHealthCheckID parses a health check ID string into its component parts
// Format can be "configId:workflowId" or just "actionId" (legacy)
func ParseHealthCheckID(id string) HealthCheckIDPair {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) == 2 {
		return HealthCheckIDPair{
			ConfigID:   parts[0],
			WorkflowID: parts[1],
		}
	}
	// Legacy format: just the action ID (actually workflow ID)
	// For backwards compatibility, treat as workflow ID (will fail to trigger, but status will work)
	return HealthCheckIDPair{
		ConfigID:   id,
		WorkflowID: id,
	}
}

// GetHealthCheckIDPairs returns parsed health check ID pairs
func (il *InstallLink) GetHealthCheckIDPairs() []HealthCheckIDPair {
	ids := il.GetHealthCheckActionIDs()
	pairs := make([]HealthCheckIDPair, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			pairs = append(pairs, ParseHealthCheckID(id))
		}
	}
	return pairs
}

// SetVendorInputs stores vendor inputs as JSON
func (il *InstallLink) SetVendorInputs(inputs map[string]string) error {
	if inputs == nil || len(inputs) == 0 {
		il.VendorInputs = ""
		return nil
	}
	data, err := json.Marshal(inputs)
	if err != nil {
		return err
	}
	il.VendorInputs = string(data)
	return nil
}

// GetVendorInputs retrieves vendor inputs from JSON
func (il *InstallLink) GetVendorInputs() (map[string]string, error) {
	if il.VendorInputs == "" {
		return make(map[string]string), nil
	}
	var inputs map[string]string
	if err := json.Unmarshal([]byte(il.VendorInputs), &inputs); err != nil {
		return nil, err
	}
	return inputs, nil
}
