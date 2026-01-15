package overrides

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"gorm.io/gorm"
)

// TemplateRenderer handles rendering customer pages with optional template overrides.
// It checks for workspace-specific template overrides and falls back to default Templ templates.
type TemplateRenderer struct {
	db *gorm.DB

	// Template cache: map[workspaceID:pageName]*template.Template
	cache   map[string]*template.Template
	cacheMu sync.RWMutex
}

// NewTemplateRenderer creates a new TemplateRenderer.
func NewTemplateRenderer(db *gorm.DB) *TemplateRenderer {
	return &TemplateRenderer{
		db:    db,
		cache: make(map[string]*template.Template),
	}
}

// RenderResult represents the result of a render attempt.
type RenderResult struct {
	// Rendered indicates if an override template was used.
	Rendered bool
	// Error is set if rendering failed.
	Error error
}

// TryRender attempts to render a page using an override template.
// Returns (true, nil) if an override was rendered successfully.
// Returns (false, nil) if no override exists (caller should use default template).
// Returns (false, error) if an override exists but rendering failed.
func (r *TemplateRenderer) TryRender(c *gin.Context, workspaceID, pageName string, ctx *TemplateContext) RenderResult {
	if workspaceID == "" {
		return RenderResult{Rendered: false}
	}

	// Check for enabled override
	override, err := models.GetEnabledTemplateOverride(r.db, workspaceID, pageName)
	if err != nil {
		// No override found - use default
		return RenderResult{Rendered: false}
	}

	// Get or parse the template
	tmpl, err := r.getOrParseTemplate(workspaceID, pageName, override.Content)
	if err != nil {
		return RenderResult{Rendered: false, Error: fmt.Errorf("failed to parse template: %w", err)}
	}

	// Render to buffer first to catch errors
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return RenderResult{Rendered: false, Error: fmt.Errorf("failed to execute template: %w", err)}
	}

	// Write to response
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, &buf)

	return RenderResult{Rendered: true}
}

// TryRenderWithStatus is like TryRender but allows specifying an HTTP status code.
func (r *TemplateRenderer) TryRenderWithStatus(c *gin.Context, status int, workspaceID, pageName string, ctx *TemplateContext) RenderResult {
	if workspaceID == "" {
		return RenderResult{Rendered: false}
	}

	// Check for enabled override
	override, err := models.GetEnabledTemplateOverride(r.db, workspaceID, pageName)
	if err != nil {
		// No override found - use default
		return RenderResult{Rendered: false}
	}

	// Get or parse the template
	tmpl, err := r.getOrParseTemplate(workspaceID, pageName, override.Content)
	if err != nil {
		return RenderResult{Rendered: false, Error: fmt.Errorf("failed to parse template: %w", err)}
	}

	// Render to buffer first to catch errors
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return RenderResult{Rendered: false, Error: fmt.Errorf("failed to execute template: %w", err)}
	}

	// Write to response
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(status)
	_, _ = io.Copy(c.Writer, &buf)

	return RenderResult{Rendered: true}
}

// getOrParseTemplate returns a cached template or parses and caches a new one.
func (r *TemplateRenderer) getOrParseTemplate(workspaceID, pageName, content string) (*template.Template, error) {
	cacheKey := workspaceID + ":" + pageName

	// Try cache first
	r.cacheMu.RLock()
	if tmpl, ok := r.cache[cacheKey]; ok {
		r.cacheMu.RUnlock()
		return tmpl, nil
	}
	r.cacheMu.RUnlock()

	// Parse template
	tmpl, err := template.New(pageName).Funcs(templateFuncs()).Parse(content)
	if err != nil {
		return nil, err
	}

	// Cache it
	r.cacheMu.Lock()
	r.cache[cacheKey] = tmpl
	r.cacheMu.Unlock()

	return tmpl, nil
}

// InvalidateCache removes a template from the cache.
// Call this when a template override is updated or deleted.
func (r *TemplateRenderer) InvalidateCache(workspaceID, pageName string) {
	cacheKey := workspaceID + ":" + pageName
	r.cacheMu.Lock()
	delete(r.cache, cacheKey)
	r.cacheMu.Unlock()
}

// InvalidateWorkspaceCache removes all templates for a workspace from the cache.
// Call this when templates are synced from GitHub.
func (r *TemplateRenderer) InvalidateWorkspaceCache(workspaceID string) {
	r.cacheMu.Lock()
	for key := range r.cache {
		// Keys are in format "workspaceID:pageName"
		if len(key) > len(workspaceID)+1 && key[:len(workspaceID)+1] == workspaceID+":" {
			delete(r.cache, key)
		}
	}
	r.cacheMu.Unlock()
}

// GetCustomCSSPath returns the URL path for custom CSS if it exists for the workspace.
func (r *TemplateRenderer) GetCustomCSSPath(workspaceID, basePath string) string {
	if workspaceID == "" {
		return ""
	}

	// Check if any CSS overrides exist
	cssAssets, err := models.GetCSSOverrides(r.db, workspaceID)
	if err != nil || len(cssAssets) == 0 {
		return ""
	}

	// Return the path to serve custom CSS
	return basePath + "/custom/css/" + workspaceID + ".css"
}

// ValidateTemplate validates a template string to ensure it parses correctly
// with all available template functions (including component functions).
// This should be used by the syncer when validating templates from GitHub.
func ValidateTemplate(name, content string) error {
	_, err := template.New(name).Funcs(templateFuncs()).Parse(content)
	return err
}

// templateFuncs returns template functions available in override templates.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		// Safe URL output
		"safeURL": func(s string) template.URL {
			return template.URL(s)
		},
		// Safe HTML output (use with caution)
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		// Safe CSS output
		"safeCSS": func(s string) template.CSS {
			return template.CSS(s)
		},
		// Check if a value is empty
		"empty": func(v interface{}) bool {
			if v == nil {
				return true
			}
			switch val := v.(type) {
			case string:
				return val == ""
			case []interface{}:
				return len(val) == 0
			default:
				return false
			}
		},
		// Simple conditional helper
		"default": func(defaultVal, val interface{}) interface{} {
			if val == nil || val == "" {
				return defaultVal
			}
			return val
		},

		// Component functions - render pre-defined Templ components
		// Usage: {{ statusBadge .Status }} or {{ statusBadge .Status "install" "md" }}
		"statusBadge": RenderStatusBadge,
		// Usage: {{ alert "info" "Message" }} or {{ alert "error" .ErrorMessage }}
		"alert": RenderAlert,
		// Usage: {{ emptyState "Title" "Message" }} or {{ emptyState "Title" "Message" "icon" }}
		"emptyState": RenderEmptyState,
		// Usage: {{ pagination .PageData }} (when PageData has pagination fields)
		"pagination": RenderPagination,
		// Helper to create pagination data: {{ pagination (paginationData 1 10 false true 0 2 "?page=") }}
		"paginationData": MakePaginationData,

		// Layout functions - render standard layout components
		// Usage: {{ head . }} - renders complete <head> section with CSS, fonts, theme
		"head": RenderHead,
		// Usage: <body {{ bodyAttrs . }}> - renders body attributes with theme classes
		"bodyAttrs": RenderBodyAttrs,
		// Usage: {{ header . }} - renders header with logo, user, logout
		"header": RenderHeader,
		// Usage: {{ footer . }} - renders footer with branding
		"footer": RenderFooter,
		// Usage: {{ scripts . }} - renders utility scripts (toast, modals, auth)
		"scripts": RenderScripts,
		// Usage: {{ previewBanner }} - renders dismissible preview banner
		"previewBanner": RenderPreviewBanner,
	}
}
