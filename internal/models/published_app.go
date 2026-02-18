package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

// PublishedApp represents a Nuon app that has been published for customers to discover and install
// without needing a specific install link.
type PublishedApp struct {
	ID        string         `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	OrgID     string         `gorm:"not null;index" json:"org_id"` // FK to NuonOrg.ID (local)
	AppID     string         `gorm:"not null" json:"app_id"`       // Nuon API app ID
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	NuonOrg NuonOrg `gorm:"foreignKey:OrgID" json:"nuon_org,omitempty"`
}

func (p *PublishedApp) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = shortid.NewPublishedAppID()
	}
	return nil
}
