package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nuonco/mono/services/customer-dashboard/internal/github"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

// GetGitHubConfig returns the GitHub repo configuration for the current workspace.
func (h *Handler) GetGitHubConfig(c *gin.Context) {
	workspace := middleware.GetCurrentWorkspace(c)
	if workspace == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace context not found"})
		return
	}

	config, err := models.GetGitHubRepoConfig(h.db, workspace.ID)
	if err != nil {
		// Not found is OK - just means not configured yet
		c.JSON(http.StatusOK, gin.H{
			"configured": false,
		})
		return
	}

	// Get template overrides for this workspace
	templates, _ := models.GetAllTemplateOverrides(h.db, workspace.ID)
	assets, _ := models.GetAllAssetOverrides(h.db, workspace.ID)

	c.JSON(http.StatusOK, gin.H{
		"configured":       true,
		"repo_owner":       config.RepoOwner,
		"repo_name":        config.RepoName,
		"branch":           config.Branch,
		"last_sync_at":     config.LastSyncAt,
		"last_sync_status": config.LastSyncStatus,
		"last_sync_error":  config.LastSyncError,
		"last_sync_sha":    config.LastSyncSHA,
		"templates":        templates,
		"assets":           assets,
	})
}

// SaveGitHubConfig saves or updates the GitHub repo configuration.
func (h *Handler) SaveGitHubConfig(c *gin.Context) {
	workspace := middleware.GetCurrentWorkspace(c)
	if workspace == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace context not found"})
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
	config, err := models.GetGitHubRepoConfig(h.db, workspace.ID)
	if err != nil {
		// Create new
		config = &models.GitHubRepoConfig{
			WorkspaceID: workspace.ID,
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
	workspace := middleware.GetCurrentWorkspace(c)
	if workspace == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace context not found"})
		return
	}

	config, err := models.GetGitHubRepoConfig(h.db, workspace.ID)
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
	workspace := middleware.GetCurrentWorkspace(c)
	if workspace == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace context not found"})
		return
	}

	// Delete all template overrides
	if err := h.db.Where("workspace_id = ?", workspace.ID).Delete(&models.TemplateOverride{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete template overrides"})
		return
	}

	// Delete all asset overrides
	if err := h.db.Where("workspace_id = ?", workspace.ID).Delete(&models.AssetOverride{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete asset overrides"})
		return
	}

	// Delete the config
	if err := h.db.Where("workspace_id = ?", workspace.ID).Delete(&models.GitHubRepoConfig{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "GitHub integration removed"})
}

// ToggleTemplateOverride enables or disables a specific template override.
func (h *Handler) ToggleTemplateOverride(c *gin.Context) {
	workspace := middleware.GetCurrentWorkspace(c)
	if workspace == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace context not found"})
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

	override, err := models.GetTemplateOverride(h.db, workspace.ID, pageName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template override not found"})
		return
	}

	override.IsEnabled = req.Enabled
	if err := h.db.Save(override).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update override"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Template override updated",
		"page_name":  pageName,
		"is_enabled": override.IsEnabled,
	})
}

// DeleteTemplateOverride removes a specific template override.
func (h *Handler) DeleteTemplateOverride(c *gin.Context) {
	workspace := middleware.GetCurrentWorkspace(c)
	if workspace == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace context not found"})
		return
	}

	pageName := c.Param("page")
	if !models.IsValidPageName(pageName) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page name"})
		return
	}

	if err := h.db.Where("workspace_id = ? AND page_name = ?", workspace.ID, pageName).Delete(&models.TemplateOverride{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete override"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Template override deleted"})
}

// ServeCustomCSS serves the combined custom CSS for a workspace.
func (h *Handler) ServeCustomCSS(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	if workspaceID == "" {
		c.Status(http.StatusNotFound)
		return
	}

	// Remove .css extension if present
	if len(workspaceID) > 4 && workspaceID[len(workspaceID)-4:] == ".css" {
		workspaceID = workspaceID[:len(workspaceID)-4]
	}

	// Get all enabled CSS overrides for this workspace
	cssAssets, err := models.GetCSSOverrides(h.db, workspaceID)
	if err != nil || len(cssAssets) == 0 {
		c.Status(http.StatusNotFound)
		return
	}

	// Combine all CSS
	c.Header("Content-Type", "text/css; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	c.Status(http.StatusOK)

	for _, asset := range cssAssets {
		c.Writer.Write([]byte("/* " + asset.SourcePath + " */\n"))
		c.Writer.Write(asset.Content)
		c.Writer.Write([]byte("\n\n"))
	}
}

// ServeCustomAsset serves a custom asset (image) for a workspace.
func (h *Handler) ServeCustomAsset(c *gin.Context) {
	workspaceID := c.Param("workspace_id")
	assetPath := c.Param("path")

	if workspaceID == "" || assetPath == "" {
		c.Status(http.StatusNotFound)
		return
	}

	// Remove leading slash from path
	if len(assetPath) > 0 && assetPath[0] == '/' {
		assetPath = assetPath[1:]
	}

	asset, err := models.GetEnabledAssetOverride(h.db, workspaceID, assetPath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Content-Type", asset.MimeType)
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 24 hours
	c.Data(http.StatusOK, asset.MimeType, asset.Content)
}
