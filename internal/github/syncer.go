package github

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/overrides"
)

// Syncer handles syncing template overrides from GitHub repositories.
type Syncer struct {
	db     *gorm.DB
	client *Client
}

// NewSyncer creates a new Syncer.
func NewSyncer(db *gorm.DB) *Syncer {
	return &Syncer{
		db:     db,
		client: NewClient(),
	}
}

// SyncResult contains the result of a sync operation.
type SyncResult struct {
	TemplatesSynced  int
	AssetsSynced     int
	TemplatesRemoved int
	AssetsRemoved    int
	Errors           []string
}

// File size limits
const (
	MaxTemplateSize = 500 * 1024      // 500KB
	MaxAssetSize    = 100 * 1024      // 100KB
	MaxTotalSize    = 2 * 1024 * 1024 // 2MB
)

// Sync performs a full sync from the configured GitHub repository.
func (s *Syncer) Sync(ctx context.Context, config *models.GitHubRepoConfig) (*SyncResult, error) {
	result := &SyncResult{}

	// Update status to pending
	now := time.Now()
	config.LastSyncAt = &now
	config.LastSyncStatus = models.SyncStatusPending
	config.LastSyncError = ""
	if err := s.db.Save(config).Error; err != nil {
		return nil, fmt.Errorf("failed to update sync status: %w", err)
	}

	// Fetch repository tree
	tree, err := s.client.GetTree(ctx, config.RepoOwner, config.RepoName, config.Branch, config.AccessToken)
	if err != nil {
		s.updateSyncStatus(config, models.SyncStatusFailed, fmt.Sprintf("Failed to fetch repository tree: %v", err), "", nil)
		return nil, fmt.Errorf("failed to fetch tree: %w", err)
	}

	// Fetch commit details
	var commit *Commit
	commit, err = s.client.GetCommit(ctx, config.RepoOwner, config.RepoName, tree.SHA, config.AccessToken)
	if err != nil {
		// Log error but don't fail the sync - commit metadata is supplementary
		zap.L().Warn("failed to fetch commit details", zap.Error(err))
	}

	// Discover files by convention
	discoveredTemplates := make(map[string]*TreeEntry) // pageName -> entry
	discoveredAssets := make(map[string]*TreeEntry)    // assetPath -> entry
	var totalSize int64

	for i := range tree.Tree {
		entry := &tree.Tree[i]
		if entry.Type != "blob" {
			continue
		}

		switch {
		case strings.HasPrefix(entry.Path, "pages/") && strings.HasSuffix(entry.Path, ".html"):
			// Template file: pages/login.html -> "login"
			pageName := strings.TrimSuffix(filepath.Base(entry.Path), ".html")
			if models.IsValidPageName(pageName) {
				if entry.Size > MaxTemplateSize {
					result.Errors = append(result.Errors, fmt.Sprintf("Template %s exceeds size limit (%d > %d)", entry.Path, entry.Size, MaxTemplateSize))
					continue
				}
				discoveredTemplates[pageName] = entry
				totalSize += entry.Size
			}

		case strings.HasPrefix(entry.Path, "partials/") && strings.HasSuffix(entry.Path, ".html"):
			// Partial template file: partials/header.html -> "header"
			partialName := strings.TrimSuffix(filepath.Base(entry.Path), ".html")
			if models.IsValidPartialName(partialName) {
				if entry.Size > MaxTemplateSize {
					result.Errors = append(result.Errors, fmt.Sprintf("Partial %s exceeds size limit (%d > %d)", entry.Path, entry.Size, MaxTemplateSize))
					continue
				}
				discoveredTemplates[partialName] = entry
				totalSize += entry.Size
			}

		case strings.HasPrefix(entry.Path, "css/") && strings.HasSuffix(entry.Path, ".css"):
			// CSS file
			if entry.Size > MaxAssetSize {
				result.Errors = append(result.Errors, fmt.Sprintf("CSS %s exceeds size limit (%d > %d)", entry.Path, entry.Size, MaxAssetSize))
				continue
			}
			assetPath := strings.TrimPrefix(entry.Path, "css/")
			discoveredAssets[assetPath] = entry
			totalSize += entry.Size

		case strings.HasPrefix(entry.Path, "assets/"):
			// Asset file (images)
			ext := strings.ToLower(filepath.Ext(entry.Path))
			if !models.IsAllowedAssetExtension(ext) {
				continue
			}
			if entry.Size > MaxAssetSize {
				result.Errors = append(result.Errors, fmt.Sprintf("Asset %s exceeds size limit (%d > %d)", entry.Path, entry.Size, MaxAssetSize))
				continue
			}
			assetPath := strings.TrimPrefix(entry.Path, "assets/")
			discoveredAssets[assetPath] = entry
			totalSize += entry.Size
		}
	}

	// Check total size limit
	if totalSize > MaxTotalSize {
		s.updateSyncStatus(config, models.SyncStatusFailed, fmt.Sprintf("Total size exceeds limit (%d > %d)", totalSize, MaxTotalSize), "", nil)
		return nil, fmt.Errorf("total size exceeds limit: %d > %d", totalSize, MaxTotalSize)
	}

	// Sync templates
	for pageName, entry := range discoveredTemplates {
		if err := s.syncTemplate(ctx, config, pageName, entry, tree.SHA); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to sync template %s: %v", pageName, err))
			continue
		}
		result.TemplatesSynced++
	}

	// Sync assets
	for assetPath, entry := range discoveredAssets {
		if err := s.syncAsset(ctx, config, assetPath, entry, tree.SHA); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to sync asset %s: %v", assetPath, err))
			continue
		}
		result.AssetsSynced++
	}

	// Remove templates that no longer exist in repo
	existingTemplates, _ := models.GetAllTemplateOverrides(s.db, config.OrgID)
	for _, tmpl := range existingTemplates {
		if _, exists := discoveredTemplates[tmpl.PageName]; !exists {
			if err := s.db.Delete(&tmpl).Error; err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to remove template %s: %v", tmpl.PageName, err))
			} else {
				result.TemplatesRemoved++
			}
		}
	}

	// Remove assets that no longer exist in repo
	existingAssets, _ := models.GetAllAssetOverrides(s.db, config.OrgID)
	for _, asset := range existingAssets {
		if _, exists := discoveredAssets[asset.AssetPath]; !exists {
			if err := s.db.Delete(&asset).Error; err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to remove asset %s: %v", asset.AssetPath, err))
			} else {
				result.AssetsRemoved++
			}
		}
	}

	// Update sync status
	status := models.SyncStatusSuccess
	syncError := ""
	if len(result.Errors) > 0 {
		status = models.SyncStatusFailed
		syncError = strings.Join(result.Errors, "; ")
	}
	s.updateSyncStatus(config, status, syncError, tree.SHA, commit)

	return result, nil
}

// syncTemplate fetches and stores a single template override.
func (s *Syncer) syncTemplate(ctx context.Context, config *models.GitHubRepoConfig, pageName string, entry *TreeEntry, commitSHA string) error {
	// Fetch content
	content, err := s.client.GetFileContent(ctx, config.RepoOwner, config.RepoName, entry.Path, config.Branch, config.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to fetch content: %w", err)
	}

	// Validate template syntax (including component functions like statusBadge, alert, etc.)
	if err := overrides.ValidateTemplate(pageName, string(content)); err != nil {
		return fmt.Errorf("invalid template syntax: %w", err)
	}

	// Upsert template override
	var existing models.TemplateOverride
	err = s.db.Where("workspace_id = ? AND page_name = ?", config.OrgID, pageName).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// Create new
		override := models.TemplateOverride{
			OrgID:      config.OrgID,
			PageName:   pageName,
			Content:    string(content),
			SourcePath: entry.Path,
			SourceSHA:  commitSHA,
			IsEnabled:  true,
		}
		return s.db.Create(&override).Error
	} else if err != nil {
		return err
	}

	// Update existing
	existing.Content = string(content)
	existing.SourcePath = entry.Path
	existing.SourceSHA = commitSHA
	return s.db.Save(&existing).Error
}

// syncAsset fetches and stores a single asset override.
func (s *Syncer) syncAsset(ctx context.Context, config *models.GitHubRepoConfig, assetPath string, entry *TreeEntry, commitSHA string) error {
	// Determine asset type and MIME type
	ext := strings.ToLower(filepath.Ext(entry.Path))
	mimeType, ok := models.GetMimeTypeForExtension(ext)
	if !ok {
		return fmt.Errorf("unsupported file type: %s", ext)
	}

	assetType := models.AssetTypeImage
	if ext == ".css" {
		assetType = models.AssetTypeCSS
	}

	// Fetch content (use raw for binary files)
	var content []byte
	var err error
	if assetType == models.AssetTypeCSS {
		content, err = s.client.GetFileContent(ctx, config.RepoOwner, config.RepoName, entry.Path, config.Branch, config.AccessToken)
	} else {
		content, err = s.client.GetRawFileContent(ctx, config.RepoOwner, config.RepoName, entry.Path, config.Branch, config.AccessToken)
	}
	if err != nil {
		return fmt.Errorf("failed to fetch content: %w", err)
	}

	// CSS sanitization (basic - remove potentially dangerous content)
	if assetType == models.AssetTypeCSS {
		content = sanitizeCSS(content)
	}

	// Upsert asset override
	var existing models.AssetOverride
	err = s.db.Where("workspace_id = ? AND asset_path = ?", config.OrgID, assetPath).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// Create new
		asset := models.AssetOverride{
			OrgID:      config.OrgID,
			AssetType:  assetType,
			AssetPath:  assetPath,
			Content:    content,
			MimeType:   mimeType,
			FileSize:   int64(len(content)),
			SourcePath: entry.Path,
			SourceSHA:  commitSHA,
			IsEnabled:  true,
		}
		return s.db.Create(&asset).Error
	} else if err != nil {
		return err
	}

	// Update existing
	existing.Content = content
	existing.MimeType = mimeType
	existing.FileSize = int64(len(content))
	existing.SourcePath = entry.Path
	existing.SourceSHA = commitSHA
	return s.db.Save(&existing).Error
}

// updateSyncStatus updates the sync status on the config.
func (s *Syncer) updateSyncStatus(config *models.GitHubRepoConfig, status, errorMsg, sha string, commit *Commit) {
	now := time.Now()
	config.LastSyncAt = &now
	config.LastSyncStatus = status
	config.LastSyncError = errorMsg
	if sha != "" {
		config.LastSyncSHA = sha
	}
	if commit != nil {
		config.LastSyncCommitMessage = commit.Commit.Message
		config.LastSyncCommitAuthor = commit.Commit.Author.Name
		config.LastSyncCommitDate = &commit.Commit.Author.Date
	}
	s.db.Save(config)
}

// sanitizeCSS removes potentially dangerous CSS content.
func sanitizeCSS(content []byte) []byte {
	css := string(content)

	// Remove @import statements
	lines := strings.Split(css, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.ToLower(line))
		if strings.HasPrefix(trimmed, "@import") {
			continue
		}
		cleanLines = append(cleanLines, line)
	}
	css = strings.Join(cleanLines, "\n")

	// Remove javascript: URLs
	css = strings.ReplaceAll(css, "javascript:", "")

	// Remove expression() (IE-specific)
	// Simple approach - a more robust solution would use a CSS parser
	for strings.Contains(strings.ToLower(css), "expression(") {
		lower := strings.ToLower(css)
		idx := strings.Index(lower, "expression(")
		if idx == -1 {
			break
		}
		// Find matching closing paren
		depth := 1
		end := idx + 11
		for end < len(css) && depth > 0 {
			if css[end] == '(' {
				depth++
			} else if css[end] == ')' {
				depth--
			}
			end++
		}
		css = css[:idx] + css[end:]
	}

	return []byte(css)
}
