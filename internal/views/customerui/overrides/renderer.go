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
// It checks for org-specific template overrides and falls back to default Templ templates.
type TemplateRenderer struct {
	db *gorm.DB

	// Template cache: map[orgID:pageName]*template.Template
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
func (r *TemplateRenderer) TryRender(c *gin.Context, orgID, pageName string, ctx *TemplateContext) RenderResult {
	if orgID == "" {
		return RenderResult{Rendered: false}
	}

	// Check for enabled override
	override, err := models.GetEnabledTemplateOverride(r.db, orgID, pageName)
	if err != nil {
		// Debug: Log when no override is found
		fmt.Printf("[DEBUG] TryRender: no override found for org=%s, page=%s, err=%v\n",
			orgID, pageName, err)
		// No override found - use default
		return RenderResult{Rendered: false}
	}

	// Debug: Log that override was found
	fmt.Printf("[DEBUG] TryRender: found override for org=%s, page=%s, id=%s\n",
		orgID, pageName, override.ID)

	// Get or parse the template
	tmpl, err := r.getOrParseTemplate(orgID, pageName, override.Content)
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
func (r *TemplateRenderer) TryRenderWithStatus(c *gin.Context, status int, orgID, pageName string, ctx *TemplateContext) RenderResult {
	if orgID == "" {
		return RenderResult{Rendered: false}
	}

	// Check for enabled override
	override, err := models.GetEnabledTemplateOverride(r.db, orgID, pageName)
	if err != nil {
		// No override found - use default
		return RenderResult{Rendered: false}
	}

	// Get or parse the template
	tmpl, err := r.getOrParseTemplate(orgID, pageName, override.Content)
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
// It also loads and parses any enabled partial templates for the org,
// making them available for inclusion via {{ template "partial.html" . }}.
func (r *TemplateRenderer) getOrParseTemplate(orgID, pageName, content string) (*template.Template, error) {
	cacheKey := orgID + ":" + pageName

	// Try cache first
	r.cacheMu.RLock()
	if tmpl, ok := r.cache[cacheKey]; ok {
		r.cacheMu.RUnlock()
		return tmpl, nil
	}
	r.cacheMu.RUnlock()

	// Parse main template
	tmpl, err := template.New(pageName).Funcs(r.getFuncMap()).Parse(content)
	if err != nil {
		return nil, err
	}

	// Load and parse any enabled partials for this org
	partials, err := models.GetEnabledPartials(r.db, orgID)
	if err == nil && len(partials) > 0 {
		for _, partial := range partials {
			// Register partial with .html suffix to match template references like {{ template "modal.html" . }}
			partialName := partial.PageName + ".html"
			_, err := tmpl.New(partialName).Parse(partial.Content)
			if err != nil {
				fmt.Printf("[DEBUG] getOrParseTemplate: failed to parse partial %s: %v\n", partialName, err)
				// Continue with other partials - don't fail the whole template
				continue
			}
			fmt.Printf("[DEBUG] getOrParseTemplate: loaded partial %s for org %s\n", partialName, orgID)
		}
	}

	// Cache it
	r.cacheMu.Lock()
	r.cache[cacheKey] = tmpl
	r.cacheMu.Unlock()

	return tmpl, nil
}

// InvalidateCache removes a template from the cache.
// Call this when a template override is updated or deleted.
func (r *TemplateRenderer) InvalidateCache(orgID, pageName string) {
	cacheKey := orgID + ":" + pageName
	r.cacheMu.Lock()
	delete(r.cache, cacheKey)
	r.cacheMu.Unlock()
}

// InvalidateOrgCache removes all templates for an org from the cache.
// Call this when templates are synced from GitHub.
func (r *TemplateRenderer) InvalidateOrgCache(orgID string) {
	r.cacheMu.Lock()
	for key := range r.cache {
		// Keys are in format "orgID:pageName"
		if len(key) > len(orgID)+1 && key[:len(orgID)+1] == orgID+":" {
			delete(r.cache, key)
		}
	}
	r.cacheMu.Unlock()
}

// InvalidateWorkspaceCache is deprecated - use InvalidateOrgCache instead.
// Kept temporarily for backwards compatibility during migration.
func (r *TemplateRenderer) InvalidateWorkspaceCache(orgID string) {
	r.InvalidateOrgCache(orgID)
}

// GetCustomCSSPath returns the URL path for custom CSS if it exists for the org.
func (r *TemplateRenderer) GetCustomCSSPath(orgID, basePath string) string {
	if orgID == "" {
		return ""
	}

	// Check if any CSS overrides exist
	cssAssets, err := models.GetCSSOverrides(r.db, orgID)
	if err != nil || len(cssAssets) == 0 {
		return ""
	}

	// Return the path to serve custom CSS
	return basePath + "/custom/css/" + orgID + ".css"
}

// ValidateTemplate validates a template string to ensure it parses correctly
// with all available template functions (including component functions).
// This should be used by the syncer when validating templates from GitHub.
func ValidateTemplate(name, content string) error {
	_, err := template.New(name).Funcs(getBaseFuncMap()).Parse(content)
	return err
}

// getFuncMap returns template functions available in override templates.
// This is a method on TemplateRenderer so layout functions can access the renderer.
func (r *TemplateRenderer) getFuncMap() template.FuncMap {
	funcMap := getBaseFuncMap()

	// Override layout functions to pass renderer instance
	funcMap["header"] = func(ctx *TemplateContext) template.HTML {
		return RenderHeader(ctx, r)
	}
	funcMap["sidebar"] = func(ctx *TemplateContext) template.HTML {
		return RenderSidebar(ctx, r)
	}
	funcMap["footer"] = func(ctx *TemplateContext) template.HTML {
		return RenderFooter(ctx, r)
	}
	funcMap["scripts"] = func(ctx *TemplateContext) template.HTML {
		return RenderScripts(ctx, r)
	}

	return funcMap
}

// getBaseFuncMap returns the base template functions that don't need renderer access.
func getBaseFuncMap() template.FuncMap {
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
		// String prefix check
		"hasPrefix": func(s, prefix string) bool {
			return len(s) >= len(prefix) && s[:len(prefix)] == prefix
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

		// Layout functions - render standard layout components (without override support)
		// Note: These will be overridden in getFuncMap() to add renderer support
		// Usage: {{ head . }} - renders complete <head> section with CSS, fonts, theme
		"head": RenderHead,
		// Usage: <body {{ bodyAttrs . }}> - renders body attributes with theme classes
		"bodyAttrs": RenderBodyAttrs,
		// Usage: {{ previewBanner }} - renders dismissible preview banner
		"previewBanner": RenderPreviewBanner,
		// Stub versions of layout functions for validation (will be overridden with full versions in getFuncMap)
		"header": func(ctx *TemplateContext) template.HTML {
			return template.HTML("<!-- header placeholder -->")
		},
		"sidebar": func(ctx *TemplateContext) template.HTML {
			return template.HTML("<!-- sidebar placeholder -->")
		},
		"footer": func(ctx *TemplateContext) template.HTML {
			return template.HTML("<!-- footer placeholder -->")
		},
		"scripts": func(ctx *TemplateContext) template.HTML {
			return template.HTML("<!-- scripts placeholder -->")
		},
	}
}
