package templates

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed templates/*
var templateFS embed.FS

// Renderer handles template rendering with caching and layout composition
type Renderer struct {
	templates map[string]*template.Template
	funcs     template.FuncMap
	devMode   bool
	mu        sync.RWMutex
}

// NewRenderer creates a new template renderer
// devMode=true disables caching for hot-reloading during development
func NewRenderer(devMode bool) (*Renderer, error) {
	r := &Renderer{
		templates: make(map[string]*template.Template),
		funcs:     DefaultFuncs(),
		devMode:   devMode,
	}

	if err := r.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return r, nil
}

// RegisterFuncs adds custom template functions (must be called before loading templates)
func (r *Renderer) RegisterFuncs(funcs template.FuncMap) {
	for name, fn := range funcs {
		r.funcs[name] = fn
	}
}

// loadTemplates parses all templates from the embedded filesystem
func (r *Renderer) loadTemplates() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Load all component and partial templates first (they're shared)
	sharedTemplates := template.New("").Funcs(r.funcs)

	// Walk through all templates and parse them
	err := fs.WalkDir(templateFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		content, err := templateFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		// Parse into shared templates collection
		_, err = sharedTemplates.Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Now create page-specific templates that can use the shared ones
	err = fs.WalkDir(templateFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		// Get the template name without the templates/ prefix and .html suffix
		name := strings.TrimPrefix(path, "templates/")
		name = strings.TrimSuffix(name, ".html")

		// Clone the shared templates and set the main template for this page
		pageTemplate, err := sharedTemplates.Clone()
		if err != nil {
			return fmt.Errorf("failed to clone templates for %s: %w", name, err)
		}

		r.templates[name] = pageTemplate

		return nil
	})

	return err
}

// Render renders a full page template with the given data
func (r *Renderer) Render(w io.Writer, name string, data interface{}) error {
	if r.devMode {
		// In dev mode, reload templates on each request
		if err := r.loadTemplates(); err != nil {
			return fmt.Errorf("failed to reload templates: %w", err)
		}
	}

	r.mu.RLock()
	tmpl, ok := r.templates[name]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("template not found: %s", name)
	}

	// Execute the template - it should define a layout that renders content
	return tmpl.Execute(w, data)
}

// RenderPartial renders a partial template (for HTMX responses)
func (r *Renderer) RenderPartial(w io.Writer, name string, data interface{}) error {
	if r.devMode {
		if err := r.loadTemplates(); err != nil {
			return fmt.Errorf("failed to reload templates: %w", err)
		}
	}

	r.mu.RLock()
	// For partials, we look for the template in any of our loaded templates
	var tmpl *template.Template
	for _, t := range r.templates {
		if t.Lookup(name) != nil {
			tmpl = t
			break
		}
	}
	r.mu.RUnlock()

	if tmpl == nil {
		return fmt.Errorf("partial template not found: %s", name)
	}

	return tmpl.ExecuteTemplate(w, name, data)
}

// RenderTemplate renders a specific named template within the template set
func (r *Renderer) RenderTemplate(w io.Writer, templateSet, templateName string, data interface{}) error {
	if r.devMode {
		if err := r.loadTemplates(); err != nil {
			return fmt.Errorf("failed to reload templates: %w", err)
		}
	}

	r.mu.RLock()
	tmpl, ok := r.templates[templateSet]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("template set not found: %s", templateSet)
	}

	return tmpl.ExecuteTemplate(w, templateName, data)
}

// TemplateNames returns a list of all loaded template names (for debugging)
func (r *Renderer) TemplateNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.templates))
	for name := range r.templates {
		names = append(names, name)
	}
	return names
}

// getLayoutForUI returns the layout template name for the given UI type
func getLayoutForUI(uiType string) string {
	switch uiType {
	case "customerui":
		return "customer-layout"
	case "vendorui":
		return "vendor-layout"
	default:
		return ""
	}
}

// extractUIType extracts the UI type from a template path
func extractUIType(templatePath string) string {
	parts := strings.Split(templatePath, string(filepath.Separator))
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
