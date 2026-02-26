package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nuonco/mono/services/customer-dashboard/internal/github"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

// GetGitHubConfig returns the GitHub repo configuration for the current org.
func (h *Handler) GetGitHubConfig(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization context not found"})
		return
	}

	config, err := models.GetGitHubRepoConfig(h.db, org.ID)
	if err != nil {
		// Not found is OK - just means not configured yet
		c.JSON(http.StatusOK, gin.H{
			"configured": false,
		})
		return
	}

	// Get template overrides for this org
	templates, _ := models.GetAllTemplateOverrides(h.db, org.ID)
	assets, _ := models.GetAllAssetOverrides(h.db, org.ID)

	c.JSON(http.StatusOK, gin.H{
		"configured":               true,
		"repo_owner":               config.RepoOwner,
		"repo_name":                config.RepoName,
		"branch":                   config.Branch,
		"last_sync_at":             config.LastSyncAt,
		"last_sync_status":         config.LastSyncStatus,
		"last_sync_error":          config.LastSyncError,
		"last_sync_sha":            config.LastSyncSHA,
		"last_sync_commit_message": config.LastSyncCommitMessage,
		"last_sync_commit_author":  config.LastSyncCommitAuthor,
		"last_sync_commit_date":    config.LastSyncCommitDate,
		"templates":                templates,
		"assets":                   assets,
	})
}

// SaveGitHubConfig saves or updates the GitHub repo configuration.
func (h *Handler) SaveGitHubConfig(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization context not found"})
		return
	}

	var req struct {
		RepoOwner   string `json:"repo_owner" binding:"required"`
		RepoName    string `json:"repo_name" binding:"required"`
		Branch      string `json:"branch"`
		AccessToken string `json:"access_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if req.Branch == "" {
		req.Branch = "main"
	}

	// Test connection first
	client := github.NewClient()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := client.TestConnection(ctx, req.RepoOwner, req.RepoName, req.Branch, req.AccessToken); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to connect to repository: " + err.Error()})
		return
	}

	// Check if config already exists
	config, err := models.GetGitHubRepoConfig(h.db, org.ID)
	if err != nil {
		// Create new
		config = &models.GitHubRepoConfig{
			OrgID:       org.ID,
			RepoOwner:   req.RepoOwner,
			RepoName:    req.RepoName,
			Branch:      req.Branch,
			AccessToken: req.AccessToken,
		}
		if err := h.db.Create(config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save configuration"})
			return
		}
	} else {
		// Update existing
		config.RepoOwner = req.RepoOwner
		config.RepoName = req.RepoName
		config.Branch = req.Branch
		config.AccessToken = req.AccessToken
		if err := h.db.Save(config).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update configuration"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "GitHub repository configured successfully",
		"config": gin.H{
			"repo_owner": config.RepoOwner,
			"repo_name":  config.RepoName,
			"branch":     config.Branch,
		},
	})
}

// SyncGitHub triggers a sync from the configured GitHub repository.
func (h *Handler) SyncGitHub(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization context not found"})
		return
	}

	config, err := models.GetGitHubRepoConfig(h.db, org.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "GitHub repository not configured"})
		return
	}

	// Run sync
	syncer := github.NewSyncer(h.db)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	result, err := syncer.Sync(ctx, config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Sync failed: " + err.Error(),
			"result": result,
		})
		return
	}

	// Invalidate template cache to pick up new/updated templates
	h.templateRenderer.InvalidateWorkspaceCache(org.ID)

	c.JSON(http.StatusOK, gin.H{
		"message":           "Sync completed",
		"templates_synced":  result.TemplatesSynced,
		"assets_synced":     result.AssetsSynced,
		"templates_removed": result.TemplatesRemoved,
		"assets_removed":    result.AssetsRemoved,
		"errors":            result.Errors,
		"last_sync_status":  config.LastSyncStatus,
	})
}

// DeleteGitHubConfig removes the GitHub configuration and all associated overrides.
func (h *Handler) DeleteGitHubConfig(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization context not found"})
		return
	}

	// Delete all template overrides
	if err := h.db.Where("org_id = ?", org.ID).Delete(&models.TemplateOverride{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete template overrides"})
		return
	}

	// Delete all asset overrides
	if err := h.db.Where("org_id = ?", org.ID).Delete(&models.AssetOverride{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete asset overrides"})
		return
	}

	// Delete the config
	if err := h.db.Where("org_id = ?", org.ID).Delete(&models.GitHubRepoConfig{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete configuration"})
		return
	}

	// Invalidate all cached templates for this org
	h.templateRenderer.InvalidateWorkspaceCache(org.ID)

	c.JSON(http.StatusOK, gin.H{"message": "GitHub integration removed"})
}

// ToggleTemplateOverride enables or disables a specific template override.
func (h *Handler) ToggleTemplateOverride(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization context not found"})
		return
	}

	pageName := c.Param("page")
	if !models.IsValidPageName(pageName) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page name"})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	override, err := models.GetTemplateOverride(h.db, org.ID, pageName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template override not found"})
		return
	}

	override.IsEnabled = req.Enabled
	if err := h.db.Save(override).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update override"})
		return
	}

	// Invalidate cache for this specific template
	h.templateRenderer.InvalidateCache(org.ID, pageName)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Template override updated",
		"page_name":  pageName,
		"is_enabled": override.IsEnabled,
	})
}

// ToggleAssetOverride enables or disables a custom asset.
func (h *Handler) ToggleAssetOverride(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization context not found"})
		return
	}

	assetPath := c.Param("path")
	// Remove leading slash if present
	if len(assetPath) > 0 && assetPath[0] == '/' {
		assetPath = assetPath[1:]
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Get the asset override
	asset, err := models.GetAssetOverride(h.db, org.ID, assetPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
		return
	}

	// Update enabled state
	asset.IsEnabled = req.Enabled
	if err := h.db.Save(asset).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update asset"})
		return
	}

	// Invalidate CSS cache if it's a CSS file
	if asset.AssetType == models.AssetTypeCSS {
		// Clear org CSS cache by invalidating all CSS assets
		h.templateRenderer.InvalidateWorkspaceCache(org.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Asset override updated",
		"asset_path": assetPath,
		"is_enabled": asset.IsEnabled,
	})
}

// BulkToggleOverrides enables or disables multiple templates and assets at once.
func (h *Handler) BulkToggleOverrides(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization context not found"})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
		All     bool `json:"all"` // If true, toggle all files
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Start transaction for atomicity
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if req.All {
		// Update all templates
		if err := tx.Model(&models.TemplateOverride{}).
			Where("org_id = ?", org.ID).
			Update("is_enabled", req.Enabled).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update templates"})
			return
		}

		// Update all assets
		if err := tx.Model(&models.AssetOverride{}).
			Where("org_id = ?", org.ID).
			Update("is_enabled", req.Enabled).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update assets"})
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit changes"})
		return
	}

	// Invalidate all caches for this org
	h.templateRenderer.InvalidateWorkspaceCache(org.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("All files %s successfully",
			map[bool]string{true: "enabled", false: "disabled"}[req.Enabled]),
		"enabled": req.Enabled,
	})
}

// DeleteTemplateOverride removes a specific template override.
func (h *Handler) DeleteTemplateOverride(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization context not found"})
		return
	}

	pageName := c.Param("page")
	if !models.IsValidPageName(pageName) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page name"})
		return
	}

	if err := h.db.Where("org_id = ? AND page_name = ?", org.ID, pageName).Delete(&models.TemplateOverride{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete override"})
		return
	}

	// Invalidate cache for this specific template
	h.templateRenderer.InvalidateCache(org.ID, pageName)

	c.JSON(http.StatusOK, gin.H{"message": "Template override deleted"})
}

// ServeCustomCSS serves the combined custom CSS for an org.
func (h *Handler) ServeCustomCSS(c *gin.Context) {
	orgID := c.Param("org_id")
	if orgID == "" {
		c.Status(http.StatusNotFound)
		return
	}

	// Remove .css extension if present
	if len(orgID) > 4 && orgID[len(orgID)-4:] == ".css" {
		orgID = orgID[:len(orgID)-4]
	}

	// Get all enabled CSS overrides for this org
	cssAssets, err := models.GetCSSOverrides(h.db, orgID)

	// Also check for theme custom CSS
	theme, themeErr := models.GetOrCreateAppTheme(h.db, orgID)
	hasThemeCSS := themeErr == nil && theme.CustomCSS != ""

	if (err != nil || len(cssAssets) == 0) && !hasThemeCSS {
		c.Status(http.StatusNotFound)
		return
	}

	// Combine all CSS
	c.Header("Content-Type", "text/css; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Status(http.StatusOK)

	for _, asset := range cssAssets {
		c.Writer.Write([]byte("/* " + asset.SourcePath + " */\n"))
		c.Writer.Write(asset.Content)
		c.Writer.Write([]byte("\n\n"))
	}

	if hasThemeCSS {
		c.Writer.Write([]byte("/* custom css */\n"))
		c.Writer.Write([]byte(theme.CustomCSS))
		c.Writer.Write([]byte("\n"))
	}
}

// ServeCustomAsset serves a custom asset (image) for an org.
func (h *Handler) ServeCustomAsset(c *gin.Context) {
	orgID := c.Param("org_id")
	assetPath := c.Param("path")

	if orgID == "" || assetPath == "" {
		c.Status(http.StatusNotFound)
		return
	}

	// Remove leading slash from path
	if len(assetPath) > 0 && assetPath[0] == '/' {
		assetPath = assetPath[1:]
	}

	asset, err := models.GetEnabledAssetOverride(h.db, orgID, assetPath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Content-Type", asset.MimeType)
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 24 hours
	c.Data(http.StatusOK, asset.MimeType, asset.Content)
}
