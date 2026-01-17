# Customer Dashboard Components

This document provides a comprehensive guide to the customer dashboard theme system and its components.

## Directory Structure

```
internal/views/customerui/
├── layout.templ                 # Main layout wrapper (simplified to 171 lines)
├── types.go                     # Shared types and LayoutProps
├── utils/                       # Theme utilities (.go files, NOT themeable)
│   └── theme.go                 # Theme CSS building functions
├── theme/                       # Themeable UI components (.templ files)
│   ├── scripts/                 # JavaScript helpers
│   │   ├── dark_mode.templ      # Dark mode detection & toggle
│   │   └── auth.templ           # Authentication helpers
│   ├── partials/                    # Partial layout components
│   │   ├── header.templ         # Header with logo & user info
│   │   ├── footer.templ         # Footer with support contact
│   │   ├── toast.templ          # Toast notifications
│   │   ├── modal.templ          # Confirmation & prompt modals
│   │   └── theme_styles.templ   # Theme CSS injection
│   ├── components/              # Reusable page components
│   │   ├── status_badge.templ   # Status indicator badges
│   │   ├── empty_state.templ    # Empty state placeholder
│   │   ├── alert.templ          # Alert/notification banners
│   │   ├── pagination.templ     # Pagination controls
│   │   ├── health_status.templ  # Health check status display
│   │   └── workflow_card.templ  # Workflow summary card
│   └── pages/                   # Full page templates
│       ├── login.templ          # Login page
│       ├── register.templ       # Registration page
│       ├── installs.templ       # Installs list page
│       ├── install_detail.templ # Single install detail
│       ├── install_link.templ   # Install link acceptance
│       ├── workflows.templ      # Workflows history page
│       └── error.templ          # Error page
└── overrides/                   # Template override system
    ├── renderer.go              # Override template renderer
    ├── context.go               # Template context & data
    ├── components.go            # Component helpers for custom templates
    └── layout_components.go     # Layout component helpers
```

## Theme Architecture

### Key Principles

1. **Complete UI Isolation**: Customer UI is completely separate from Vendor UI
2. **File Extension = Themeability**: `.templ` files are themeable, `.go` files are not
3. **Clear Boundaries**: `/theme` contains UI, `/utils` contains backend logic
4. **No Circular Dependencies**: Core components define their own prop types

### What Can Be Customized

✅ **Themeable** (in `/theme` directory):
- All `.templ` template files
- Layout, scripts, partial components, page components
- HTML structure, CSS classes, content

❌ **NOT Themeable** (outside `/theme`):
- `types.go` - Go type definitions
- `utils/theme.go` - Theme CSS generation logic
- `overrides/` - Backend override system

## Core Components

### Scripts (`theme/scripts/`)

#### Dark Mode (`dark_mode.templ`)
Automatically detects and applies user's system dark mode preference.

```go
@scripts.DarkMode()
```

#### Auth (`auth.templ`)
Provides authentication helpers and logout functionality.

```go
@scripts.Auth(basePath string)
```

### Partial Components (`theme/partials/`)

#### Header (`header.templ`)
Renders header with logo, user email, and logout button.

**Props:**
```go
type HeaderProps struct {
    Theme    *models.AppTheme
    User     *models.User
    BasePath string
}
```

**Usage:**
```go
@partials.Header(partials.HeaderProps{
    Theme: theme,
    User: user,
    BasePath: "/customer",
})
```

#### Footer (`footer.templ`)
Renders footer with "Powered by Nuon" and optional support contact.

**Props:**
```go
type FooterProps struct {
    Theme          *models.AppTheme
    SupportContact string
}
```

**Usage:**
```go
@partials.Footer(partials.FooterProps{
    Theme: theme,
    SupportContact: "support@example.com",
})
```

#### Toast (`toast.templ`)
Toast notification system with JavaScript API.

**Usage (Template):**
```go
@partials.Toast()
```

**Usage (JavaScript):**
```javascript
// Show success toast
showToast('Operation completed!', 'success');

// Show error toast
showToast('An error occurred', 'error');

// Show info toast
showToast('Please note...', 'info');
```

#### Modal (`modal.templ`)
Provides confirmation and prompt modal dialogs.

**Usage (Template):**
```go
@partials.ConfirmModal()
@partials.PromptModal()
```

**Usage (JavaScript):**
```javascript
// Confirmation dialog
showConfirmModal({
    title: 'Delete Item',
    message: 'Are you sure?',
    confirmText: 'Delete',
    cancelText: 'Cancel'
}).then(confirmed => {
    if (confirmed) {
        // User clicked confirm
    }
});

// Prompt dialog
showPromptModal({
    title: 'Enter Name',
    message: 'Please enter a name',
    placeholder: 'Item name',
    expectedValue: 'DELETE',  // Optional validation
    validationMessage: 'Type DELETE to confirm'
}).then(value => {
    if (value !== null) {
        // User entered: value
    }
});
```

#### Theme Styles (`theme_styles.templ`)
Injects CSS custom properties for theme colors and fonts.

**Props:**
```go
type ThemeStylesProps struct {
    Theme *models.AppTheme
}
```

**Usage:**
```go
@partials.ThemeStyles(partials.ThemeStylesProps{Theme: theme})
```

### Page Components (`theme/components/`)

#### Status Badge (`status_badge.templ`)
Displays colored status indicators.

**Props:**
```go
type StatusBadgeProps struct {
    Status  string  // "running", "completed", "failed", etc.
    Variant string  // "install", "workflow", "health"
    Size    string  // "sm", "md"
}
```

**Usage:**
```go
@components.StatusBadge(components.StatusBadgeProps{
    Status: "running",
    Variant: "workflow",
    Size: "md",
})
```

#### Empty State (`empty_state.templ`)
Shows placeholder when no content is available.

**Props:**
```go
type EmptyStateProps struct {
    Title   string
    Message string
    Icon    string  // "inbox", "check-circle", "refresh"
}
```

**Usage:**
```go
@components.EmptyState(components.EmptyStateProps{
    Title: "No Installs Yet",
    Message: "Create your first install to get started",
    Icon: "inbox",
})
```

#### Alert (`alert.templ`)
Displays alert/notification banners.

**Props:**
```go
type AlertProps struct {
    Type    string  // "info", "success", "warning", "error"
    Message string
}
```

**Usage:**
```go
@components.Alert(components.AlertProps{
    Type: "warning",
    Message: "Your install is updating",
})
```

#### Pagination (`pagination.templ`)
Renders pagination controls.

**Props:**
```go
type PaginationProps struct {
    CurrentPage  int
    TotalPages   int
    HasPrevious  bool
    HasNext      bool
    PreviousPage int
    NextPage     int
    BaseURL      string
}
```

**Usage:**
```go
@components.Pagination(components.PaginationProps{
    CurrentPage: 2,
    TotalPages: 10,
    HasPrevious: true,
    HasNext: true,
    PreviousPage: 1,
    NextPage: 3,
    BaseURL: "?page=",
})
```

## Theme Utilities

### Theme CSS Building (`utils/theme.go`)

Utility functions for generating theme CSS:

```go
// Build theme CSS for layout
css := utils.BuildLayoutThemeCSS(theme)

// Build theme CSS for login page (with defaults)
css := utils.BuildLoginThemeCSS(theme)

// Generate font face CSS
fontCSS := utils.BuildHeadingFontFace(base64Data)
fontCSS := utils.BuildBodyFontFace(base64Data)

// Build Google Fonts URL
url := utils.BuildGoogleFontsURL(headingFont, bodyFont, headingBase64, bodyBase64)

// Color manipulation
darkerColor := utils.DarkenColor("#FF0000", 20)  // 20% darker

// CSS class helpers
radiusClass := utils.GetRadiusClass("rounded")    // "radius-rounded"
densityClass := utils.GetDensityClass("compact")  // "density-compact"
```

## Custom Theme Development

### Creating a Custom Theme

1. **Mirror the `/theme` structure** in your custom theme repository
2. **Override only what you need** - missing templates fall back to defaults
3. **Use component helpers** from the override system

### Example Custom Page Template

```html
<!DOCTYPE html>
<html>
<head>
    <link href="{{.CSSPath}}" rel="stylesheet"/>
    {{.Components.ThemeStyles}}
</head>
<body>
    {{.Components.Header .User}}

    <main>
        <h1>{{.PageData.Title}}</h1>
        {{range .PageData.Installs}}
            {{$.Components.StatusBadge .Status "install"}}
            <p>{{.Name}}</p>
        {{end}}
    </main>

    {{.Components.Footer}}
    {{.Components.Toast}}
</body>
</html>
```

### Available Component Helpers

When using custom `html/template` templates, these helpers are available:

- `{{.Components.Header .User}}` - Render header
- `{{.Components.Footer}}` - Render footer
- `{{.Components.StatusBadge status variant}}` - Render status badge
- `{{.Components.Alert message type}}` - Render alert
- `{{.Components.EmptyState title message}}` - Render empty state
- `{{.Components.Pagination .PageData}}` - Render pagination
- `{{.Components.Toast}}` - Render toast container
- `{{.Components.ConfirmModal}}` - Render confirmation modal

## Theming Best Practices

### 1. Use CSS Custom Properties

Theme colors are available as CSS variables:

```css
/* In your custom styles */
.my-button {
    background-color: var(--theme-primary);
    border-color: var(--theme-primary);
}

.my-button:hover {
    background-color: var(--theme-primary-hover);
}

.my-link {
    color: var(--theme-secondary);
}
```

### 2. Use Theme Fonts

Font families are also CSS variables:

```css
.heading {
    font-family: var(--font-heading);
}

.body-text {
    font-family: var(--font-body);
}
```

### 3. Use Border Radius Classes

The theme system provides radius classes:

- `radius-sharp` - No border radius (0px)
- `radius-subtle` - Subtle corners (4px)
- `radius-rounded` - Rounded corners (8px, default)
- `radius-very-rounded` - Very rounded corners (12px)

Apply to containers:
```html
<div class="radius-rounded">...</div>
```

### 4. Use Density Classes

Spacing density classes control layout compactness:

- `density-compact` - Tight spacing
- `density-comfortable` - Balanced spacing (default)
- `density-spacious` - Generous spacing

These are automatically applied to the `<body>` tag.

## Migration from Old Structure

### Old → New Paths

| Old Path | New Path |
|----------|----------|
| `customerui/components/*.templ` | `customerui/theme/components/*.templ` |
| `customerui/pages/*.templ` | `customerui/theme/pages/*.templ` |
| `customerui/layout.templ` (384 lines) | `customerui/layout.templ` (171 lines) |
| Inline components in layout | `customerui/theme/partials/*.templ` |
| Inline scripts in layout | `customerui/theme/scripts/*.templ` |

### Import Updates

**Handlers:**
```go
// Old
import "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/components"
import "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/pages"

// New
import "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/components"
import "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/pages"
```

**Templates:**
```go
// Old (in .templ files)
import "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/components"

// New
import "github.com/nuonco/mono/services/customer-dashboard/internal/views/customerui/theme/components"
```

## Performance Considerations

1. **CSS Injection**: Theme CSS is injected as inline `<style>` tags for fastest loading
2. **Font Loading**: Custom fonts use `font-display: swap` for better perceived performance
3. **Component Rendering**: Templ generates efficient Go code for server-side rendering
4. **No JavaScript Frameworks**: Pure HTML + HTMX for minimal client-side overhead

## Troubleshooting

### Build Errors

**"undefined: customerui.LayoutProps"**
- ✅ Solution: LayoutProps is now in `customerui/types.go`, not in layout.templ

**"import cycle not allowed"**
- ✅ Solution: Core components define their own prop types locally

**"cannot find package"**
- ✅ Solution: Update imports from `customerui/components` to `customerui/theme/components`

### Runtime Issues

**Styles not applying**
- Check that `@partials.ThemeStyles()` is called in `<head>`
- Verify theme colors are set in `AppTheme` model

**Fonts not loading**
- Ensure font data is base64-encoded
- Check Google Fonts URL is correctly generated

**Dark mode not working**
- Verify `@scripts.DarkMode()` is called in `<head>`
- Check that dark mode CSS classes are defined

## Additional Resources

- **AppTheme Model**: `/internal/models/app_theme.go` - Theme database model
- **Handler Example**: `/internal/handlers/customer.go` - How to use layout
- **Override System**: `/internal/views/customerui/overrides/` - Custom template system
- **CSS Source**: `/src/customer.css` - Tailwind configuration

---

**Last Updated**: 2026-01-16
**Version**: 1.0.0
**Refactoring**: Customer Dashboard Theme Organization Improvement
