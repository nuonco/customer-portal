package models

import (
	"time"

	"github.com/nuonco/customer-portal/internal/shortid"
	"gorm.io/gorm"
)

const (
	AppStatusPublished   = "published"
	AppStatusComingSoon  = "coming_soon"
	AppStatusUnpublished = "unpublished"
)

// PublishedApp tracks a Nuon app's catalog status and display order.
// All apps (published, coming_soon, and unpublished) have records so sort order is preserved.
type PublishedApp struct {
	ID        string         `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	OrgID     string         `gorm:"not null;index" json:"org_id"` // FK to NuonOrg.ID (local)
	AppID     string         `gorm:"not null" json:"app_id"`       // Nuon API app ID
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Status           string  `gorm:"default:'published'" json:"status"` // "published" or "coming_soon"
	SortOrder        int     `gorm:"default:0" json:"sort_order"`
	LogoLightBase64  string  `gorm:"type:text" json:"logo_light_base64"` // base64 data URI for light mode
	LogoDarkBase64   string  `gorm:"type:text" json:"logo_dark_base64"`  // base64 data URI for dark mode
	OverviewMarkdown string  `gorm:"type:text" json:"overview_markdown"` // vendor-authored markdown overview
	NuonOrg          NuonOrg `gorm:"foreignKey:OrgID" json:"nuon_org,omitempty"`
}

func (p *PublishedApp) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = shortid.NewPublishedAppID()
	}
	return nil
}
