package models

import (
	"time"

	"github.com/nuonco/customer-portal/internal/shortid"
	"gorm.io/gorm"
)

// GitHubRepoConfig stores the GitHub repository configuration for an org's
// custom template overrides. Each org can have one GitHub repo configured.
type GitHubRepoConfig struct {
	ID                    string     `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	OrgID                 string     `gorm:"uniqueIndex" json:"org_id"` // One config per org
	RepoOwner             string     `json:"repo_owner"`                // e.g., "acme-corp"
	RepoName              string     `json:"repo_name"`                 // e.g., "customer-portal-theme"
	Branch                string     `json:"branch"`                    // default: "main"
	AccessToken           string     `json:"-"`                         // GitHub PAT (not exposed in JSON)
	LastSyncAt            *time.Time `json:"last_sync_at"`
	LastSyncStatus        string     `json:"last_sync_status"` // "success", "failed", "pending", ""
	LastSyncError         string     `json:"last_sync_error"`
	LastSyncSHA           string     `json:"last_sync_sha"`
	LastSyncCommitMessage string     `json:"last_sync_commit_message"`
	LastSyncCommitAuthor  string     `json:"last_sync_commit_author"`
	LastSyncCommitDate    *time.Time `json:"last_sync_commit_date"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`

	// Relationships
	Org NuonOrg `gorm:"foreignKey:OrgID" json:"org,omitempty"`
}

func (c *GitHubRepoConfig) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = shortid.NewGitHubRepoConfigID()
	}
	if c.Branch == "" {
		c.Branch = "main"
	}
	return nil
}

// GetRepoFullName returns the full repository name in "owner/name" format.
func (c *GitHubRepoConfig) GetRepoFullName() string {
	return c.RepoOwner + "/" + c.RepoName
}

// SyncStatus constants
const (
	SyncStatusSuccess = "success"
	SyncStatusFailed  = "failed"
	SyncStatusPending = "pending"
)

// GetGitHubRepoConfig returns the GitHubRepoConfig for an org, or nil if not configured.
func GetGitHubRepoConfig(db *gorm.DB, orgID string) (*GitHubRepoConfig, error) {
	var config GitHubRepoConfig
	if err := db.Where("org_id = ?", orgID).First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}
