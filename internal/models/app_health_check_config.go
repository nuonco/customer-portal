package models

import (
	"strings"
	"time"

	"github.com/powertoolsdev/mono/exp/installer/app/internal/shortid"
	"gorm.io/gorm"
)

// AppHealthCheckConfig stores health check configuration for an app globally.
// When install links are created for this app, the configured health checks
// are automatically applied.
type AppHealthCheckConfig struct {
	ID                   string    `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	AppID                string    `gorm:"uniqueIndex;not null" json:"app_id"` // Nuon app ID
	AppName              string    `json:"app_name"`                           // Cached app name for display
	HealthCheckActionIDs string    `json:"health_check_action_ids"`            // Comma-separated "configId:workflowId" pairs
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (c *AppHealthCheckConfig) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = shortid.NewHealthCheckConfigID()
	}
	return nil
}

// GetHealthCheckActionIDs returns the health check action IDs as a slice
// These are in "configId:workflowId" format
func (c *AppHealthCheckConfig) GetHealthCheckActionIDs() []string {
	if c.HealthCheckActionIDs == "" {
		return []string{}
	}
	return strings.Split(c.HealthCheckActionIDs, ",")
}

// SetHealthCheckActionIDs sets the health check action IDs from a slice
func (c *AppHealthCheckConfig) SetHealthCheckActionIDs(ids []string) {
	c.HealthCheckActionIDs = strings.Join(ids, ",")
}

// HasHealthChecks returns true if any health checks are configured
func (c *AppHealthCheckConfig) HasHealthChecks() bool {
	return c.HealthCheckActionIDs != ""
}

// HealthCheckCount returns the number of configured health checks
func (c *AppHealthCheckConfig) HealthCheckCount() int {
	if c.HealthCheckActionIDs == "" {
		return 0
	}
	return len(strings.Split(c.HealthCheckActionIDs, ","))
}
