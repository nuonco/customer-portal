package models

import (
	"time"

	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
	"gorm.io/gorm"
)

// GitHubRepoConfig stores the GitHub repository configuration for a workspace's
// custom template overrides. Each workspace can have one GitHub repo configured.
type GitHubRepoConfig struct {
	ID                    string     `gorm:"primarykey;check:id_checker,char_length(id)=26" json:"id"`
	WorkspaceID           string     `gorm:"uniqueIndex" json:"workspace_id"` // One config per workspace
	RepoOwner             string     `json:"repo_owner"`                      // e.g., "acme-corp"
	RepoName              string     `json:"repo_name"`                       // e.g., "customer-portal-theme"
	Branch                string     `json:"branch"`                          // default: "main"
	AccessToken           string     `json:"-"`                               // GitHub PAT (not exposed in JSON)
	LastSyncAt            *time.Time `json:"last_sync_at"`
	LastSyncStatus        string     `json:"last_sync_status"` // "success", "failed", "pending", ""
	LastSyncError         string     `json:"last_sync_error"`
	LastSyncSHA           string     `json:"last_sync_sha"`            // Git commit SHA of last sync
	LastSyncCommitMessage string     `json:"last_sync_commit_message"` // Commit message from last sync
	LastSyncCommitAuthor  string     `json:"last_sync_commit_author"`  // Commit author from last sync
	LastSyncCommitDate    *time.Time `json:"last_sync_commit_date"`    // Commit date from last sync
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`

	// Relationships
	Workspace Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
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

// GetOrCreateGitHubRepoConfig returns the GitHubRepoConfig for a workspace, or nil if not configured.
func GetGitHubRepoConfig(db *gorm.DB, workspaceID string) (*GitHubRepoConfig, error) {
	var config GitHubRepoConfig
	if err := db.Where("workspace_id = ?", workspaceID).First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}
