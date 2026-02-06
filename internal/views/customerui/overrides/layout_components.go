package overrides

import (
	"html/template"
	"strings"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

// Static HTML/script constants - matches output from layout.templ

const darkModeScript = `<script>
(function() {
	if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
		document.documentElement.classList.add('dark');
	}
	window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function(e) {
		document.documentElement.classList.toggle('dark', e.matches);
	});
})();
</script>`

const interFontLinks = `<link rel="preconnect" href="https://fonts.googleapis.com"/>
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin/>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet"/>`

const htmxScript = `<script src="https://unpkg.com/htmx.org@1.9.10"></script>`

const logoutScript = `<script>
function logout() {
	var config = document.getElementById('customer-layout-config');
	var basePath = config.dataset.basePath;
	document.cookie = "jwt=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT";
	window.location.href = basePath + "/login/";
}
</script>`

const toastHTML = `<div id="toast-container" class="fixed bottom-4 right-4 z-50 space-y-2"></div>
<script>
window.showToast = function(message, type) {
	type = type || 'info';
	var container = document.getElementById('toast-container');
	var toast = document.createElement('div');
	var bgColor = type === 'success' ? 'bg-green-500' : type === 'error' ? 'bg-red-500' : 'bg-blue-500';
	toast.className = bgColor + ' text-white px-4 py-3 rounded-lg shadow-lg transform transition-all duration-300 translate-x-full opacity-0';
	toast.textContent = message;
	container.appendChild(toast);
	setTimeout(function() {
		toast.classList.remove('translate-x-full', 'opacity-0');
	}, 10);
	setTimeout(function() {
		toast.classList.add('translate-x-full', 'opacity-0');
		setTimeout(function() {
			toast.remove();
		}, 300);
	}, 3000);
};
</script>`

const confirmModalHTML = `<div id="confirm-modal" class="fixed inset-0 bg-black/50 hidden z-50 overflow-y-auto">
<div class="flex items-center justify-center min-h-full p-4">
<div class="bg-white dark:bg-dark-grey-800 rounded-lg shadow-xl max-w-md w-full p-6 my-8">
<h3 id="confirm-title" class="text-lg font-semibold text-cool-grey-900 dark:text-white mb-2"></h3>
<p id="confirm-message" class="text-cool-grey-600 dark:text-cool-grey-400 mb-6"></p>
<div class="flex justify-end space-x-3">
<button id="confirm-cancel" class="px-4 py-2 text-sm font-medium text-cool-grey-700 dark:text-cool-grey-300 hover:bg-cool-grey-100 dark:hover:bg-dark-grey-800 rounded-lg">Cancel</button>
<button id="confirm-ok" class="px-4 py-2 text-sm font-medium text-white bg-theme-secondary hover:bg-theme-secondary-hover rounded-lg">Confirm</button>
</div>
</div>
</div>
</div>
<script>
window.showConfirmModal = function(options) {
	return new Promise(function(resolve) {
		var modal = document.getElementById('confirm-modal');
		document.getElementById('confirm-title').textContent = options.title || 'Confirm';
		document.getElementById('confirm-message').textContent = options.message || 'Are you sure?';
		document.getElementById('confirm-ok').textContent = options.confirmText || 'Confirm';
		document.getElementById('confirm-cancel').textContent = options.cancelText || 'Cancel';
		modal.classList.remove('hidden');
		document.getElementById('confirm-ok').onclick = function() {
			modal.classList.add('hidden');
			resolve(true);
		};
		document.getElementById('confirm-cancel').onclick = function() {
			modal.classList.add('hidden');
			resolve(false);
		};
	});
};
</script>`

const promptModalHTML = `<div id="prompt-modal" class="fixed inset-0 bg-black/50 hidden z-50 overflow-y-auto">
<div class="flex items-center justify-center min-h-full p-4">
<div class="bg-white dark:bg-dark-grey-800 rounded-lg shadow-xl max-w-md w-full p-6 my-8">
<h3 id="prompt-title" class="text-lg font-semibold text-cool-grey-900 dark:text-white mb-2"></h3>
<p id="prompt-message" class="text-cool-grey-600 dark:text-cool-grey-400 mb-4"></p>
<input type="text" id="prompt-input" class="w-full px-3 py-2 border border-cool-grey-300 dark:border-dark-grey-600 rounded-lg bg-white dark:bg-dark-grey-800 text-cool-grey-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500 mb-2" placeholder=""/>
<p id="prompt-error" class="text-red-600 dark:text-red-400 text-sm mb-4 hidden"></p>
<div class="flex justify-end space-x-3">
<button id="prompt-cancel" class="px-4 py-2 text-sm font-medium text-cool-grey-700 dark:text-cool-grey-300 hover:bg-cool-grey-100 dark:hover:bg-dark-grey-800 rounded-lg">Cancel</button>
<button id="prompt-ok" class="px-4 py-2 text-sm font-medium text-white bg-theme-secondary hover:bg-theme-secondary-hover rounded-lg">Confirm</button>
</div>
</div>
</div>
</div>
<script>
window.showPromptModal = function(options) {
	return new Promise(function(resolve) {
		var modal = document.getElementById('prompt-modal');
		var input = document.getElementById('prompt-input');
		var errorEl = document.getElementById('prompt-error');
		document.getElementById('prompt-title').textContent = options.title || 'Enter Value';
		document.getElementById('prompt-message').textContent = options.message || '';
		input.placeholder = options.placeholder || '';
		input.value = '';
		errorEl.classList.add('hidden');
		document.getElementById('prompt-ok').textContent = options.confirmText || 'Confirm';
		document.getElementById('prompt-cancel').textContent = options.cancelText || 'Cancel';
		modal.classList.remove('hidden');
		input.focus();
		document.getElementById('prompt-ok').onclick = function() {
			var value = input.value.trim();
			if (options.expectedValue && value !== options.expectedValue) {
				errorEl.textContent = options.validationMessage || 'Value does not match';
				errorEl.classList.remove('hidden');
				return;
			}
			modal.classList.add('hidden');
			resolve(value);
		};
		document.getElementById('prompt-cancel').onclick = function() {
			modal.classList.add('hidden');
			resolve(null);
		};
		input.onkeydown = function(e) {
			if (e.key === 'Enter') {
				document.getElementById('prompt-ok').click();
			}
		};
	});
};
</script>`

const authScript = `<script>
(function() {
	var originalFetch = window.fetch;
	window.fetch = function(url, options) {
		options = options || {};
		if (!options.credentials) {
			options.credentials = 'same-origin';
		}
		return originalFetch(url, options);
	};
})();
</script>`

// renderPartialOverride attempts to render a partial override, returns empty string if not found.
func renderPartialOverride(ctx *TemplateContext, renderer *TemplateRenderer, partialName string) template.HTML {
	if ctx == nil || renderer == nil || ctx.OrgID == "" {
		return ""
	}

	override, err := models.GetEnabledTemplateOverride(renderer.db, ctx.OrgID, partialName)
	if err != nil || override == nil {
		return ""
	}

	// Parse and execute the override template
	tmpl, err := template.New(partialName).Funcs(renderer.getFuncMap()).Parse(override.Content)
	if err != nil {
		return ""
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return ""
	}

	return template.HTML(buf.String())
}

// RenderHead generates the complete <head> section including:
// - Meta charset and viewport
// - Title from context
// - CSS link (cache-busted)
// - Custom CSS link (if present)
// - Dark mode detection script
// - Inter font (default)
// - Custom font @font-face declarations
// - Google Fonts links (if using Google fonts)
// - HTMX script
// - Theme CSS variables
//
// Usage in templates: {{ head . }}
func RenderHead(ctx *TemplateContext) template.HTML {
	var b strings.Builder

	b.WriteString("<head>\n")
	b.WriteString(`<meta charset="UTF-8"/>`)
	b.WriteString("\n")
	b.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1.0"/>`)
	b.WriteString("\n")
	b.WriteString("<title>")
	b.WriteString(template.HTMLEscapeString(ctx.Title))
	b.WriteString("</title>\n")

	// Main CSS
	b.WriteString(`<link href="`)
	b.WriteString(template.HTMLEscapeString(ctx.CSSPath))
	b.WriteString(`" rel="stylesheet"/>`)
	b.WriteString("\n")

	// Custom CSS if present
	if ctx.CustomCSSPath != "" {
		b.WriteString(`<link href="`)
		b.WriteString(template.HTMLEscapeString(ctx.CustomCSSPath))
		b.WriteString(`" rel="stylesheet"/>`)
		b.WriteString("\n")
	}

	// Dark mode script
	b.WriteString(darkModeScript)
	b.WriteString("\n")

	// Inter font (always)
	b.WriteString(interFontLinks)
	b.WriteString("\n")

	// Custom fonts
	if ctx.Theme != nil {
		if ctx.Theme.HeadingFontBase64 != "" {
			b.WriteString(buildCustomFontFace("CustomHeading", ctx.Theme.HeadingFontBase64))
			b.WriteString("\n")
		}
		if ctx.Theme.BodyFontBase64 != "" {
			b.WriteString(buildCustomFontFace("CustomBody", ctx.Theme.BodyFontBase64))
			b.WriteString("\n")
		}
		googleFonts := buildGoogleFontsLink(ctx.Theme)
		if googleFonts != "" {
			b.WriteString(googleFonts)
			b.WriteString("\n")
		}
	}

	// HTMX
	b.WriteString(htmxScript)
	b.WriteString("\n")

	// Theme CSS variables
	themeCSS := buildThemeCSS(ctx.Theme)
	if themeCSS != "" {
		b.WriteString(themeCSS)
		b.WriteString("\n")
	}

	b.WriteString("</head>")
	return template.HTML(b.String())
}

// buildCustomFontFace generates a @font-face style block for a custom font.
func buildCustomFontFace(family, base64 string) string {
	return `<style>
@font-face {
	font-family: '` + family + `';
	src: url(` + base64 + `) format('woff2');
	font-display: swap;
}
</style>`
}

// buildGoogleFontsLink generates Google Fonts link tags for heading/body fonts.
func buildGoogleFontsLink(theme *ThemeData) string {
	if theme == nil {
		return ""
	}

	headingFont := theme.HeadingFont
	bodyFont := theme.BodyFont
	headingFontBase64 := theme.HeadingFontBase64
	bodyFontBase64 := theme.BodyFontBase64

	// Only load from Google if not using custom base64 fonts
	needsHeading := headingFont != "" && headingFontBase64 == ""
	needsBody := bodyFont != "" && bodyFontBase64 == ""

	if !needsHeading && !needsBody {
		return ""
	}

	if needsHeading && needsBody && headingFont != bodyFont {
		return `<link href="https://fonts.googleapis.com/css2?family=` + headingFont + `:wght@500;600;700&family=` + bodyFont + `:wght@400;500;600&display=swap" rel="stylesheet"/>`
	} else if needsHeading {
		return `<link href="https://fonts.googleapis.com/css2?family=` + headingFont + `:wght@400;500;600;700&display=swap" rel="stylesheet"/>`
	} else if needsBody {
		return `<link href="https://fonts.googleapis.com/css2?family=` + bodyFont + `:wght@400;500;600;700&display=swap" rel="stylesheet"/>`
	}

	return ""
}

// buildThemeCSS generates CSS custom properties for the theme.
func buildThemeCSS(theme *ThemeData) string {
	if theme == nil {
		return ""
	}

	var css strings.Builder
	if theme.PrimaryColor != "" {
		css.WriteString("--theme-primary: ")
		css.WriteString(theme.PrimaryColor)
		css.WriteString(";")
		css.WriteString("--theme-primary-hover: ")
		css.WriteString(theme.PrimaryColorDark)
		css.WriteString(";")
	}
	if theme.SecondaryColor != "" {
		css.WriteString("--theme-secondary: ")
		css.WriteString(theme.SecondaryColor)
		css.WriteString(";")
		css.WriteString("--theme-secondary-hover: ")
		css.WriteString(theme.SecondaryColorDark)
		css.WriteString(";")
	}
	if theme.HeadingFontBase64 != "" {
		css.WriteString("--font-heading: 'CustomHeading', 'Inter', ui-sans-serif, system-ui, sans-serif;")
	} else if theme.HeadingFont != "" {
		css.WriteString("--font-heading: '")
		css.WriteString(theme.HeadingFont)
		css.WriteString("', 'Inter', ui-sans-serif, system-ui, sans-serif;")
	}
	if theme.BodyFontBase64 != "" {
		css.WriteString("--font-body: 'CustomBody', 'Inter', ui-sans-serif, system-ui, sans-serif;")
	} else if theme.BodyFont != "" {
		css.WriteString("--font-body: '")
		css.WriteString(theme.BodyFont)
		css.WriteString("', 'Inter', ui-sans-serif, system-ui, sans-serif;")
	}

	if css.Len() == 0 {
		return ""
	}
	return "<style>:root{" + css.String() + "}</style>"
}

// RenderBodyAttrs generates body element attributes string.
// Returns: class="bg-cool-grey-50 dark:bg-dark-grey-950 dark:text-white radius-rounded"
//
// Usage in templates: <body {{ bodyAttrs . }}>
func RenderBodyAttrs(ctx *TemplateContext) template.HTMLAttr {
	classes := []string{"bg-cool-grey-50", "dark:bg-dark-grey-950", "dark:text-white"}

	if ctx.Theme != nil {
		if ctx.Theme.RadiusClass != "" {
			classes = append(classes, ctx.Theme.RadiusClass)
		}
	}

	return template.HTMLAttr(`class="` + strings.Join(classes, " ") + `"`)
}

// RenderHeader generates the header HTML with logo, user email, and logout button.
//
// Usage in templates: {{ header . }}
func RenderHeader(ctx *TemplateContext, renderer *TemplateRenderer) template.HTML {
	// Check for partial override
	if override := renderPartialOverride(ctx, renderer, "header"); override != "" {
		return override
	}

	// Default header rendering
	var b strings.Builder

	b.WriteString(`<header class="max-w-5xl mx-auto px-6 lg:px-8 pt-6">`)
	b.WriteString(`<div class="flex items-center justify-between">`)
	b.WriteString(`<div class="flex items-center gap-4">`)

	// Logo
	if ctx.Theme != nil && ctx.Theme.LogoBase64 != "" {
		b.WriteString(`<img src="`)
		b.WriteString(ctx.Theme.LogoBase64)
		b.WriteString(`" alt="Logo" class="h-8 w-auto max-w-32 object-contain"/>`)
	}

	// User email
	if ctx.User != nil {
		b.WriteString(`<span class="text-sm text-cool-grey-600 dark:text-cool-grey-400 font-body">`)
		b.WriteString(template.HTMLEscapeString(ctx.User.Email))
		b.WriteString(`</span>`)
	}

	b.WriteString(`</div>`)

	// Logout button
	if ctx.User != nil {
		b.WriteString(`<button onclick="logout()" class="text-sm text-cool-grey-600 dark:text-cool-grey-400 hover:text-cool-grey-900 dark:hover:text-white transition-colors font-body">Logout</button>`)
	}

	b.WriteString(`</div>`)
	b.WriteString(`</header>`)

	return template.HTML(b.String())
}

// RenderSidebar generates the sidebar navigation HTML.
// This checks for a sidebar partial override and renders it if found.
// If no override exists, returns empty string (no default sidebar implementation).
//
// Usage in templates: {{ sidebar . }}
func RenderSidebar(ctx *TemplateContext, renderer *TemplateRenderer) template.HTML {
	// Check for partial override
	if override := renderPartialOverride(ctx, renderer, "sidebar"); override != "" {
		return override
	}

	// No default sidebar - return empty
	// Organizations must provide their own sidebar.html partial
	return ""
}

// RenderFooter generates the footer HTML with branding and support link.
//
// Usage in templates: {{ footer . }}
func RenderFooter(ctx *TemplateContext, renderer *TemplateRenderer) template.HTML {
	// Check for partial override
	if override := renderPartialOverride(ctx, renderer, "footer"); override != "" {
		return override
	}

	// Default footer rendering
	var b strings.Builder

	b.WriteString(`<footer class="max-w-5xl mx-auto px-6 lg:px-8 pb-6 pt-8 mt-auto">`)
	b.WriteString(`<div class="text-center text-sm text-cool-grey-500 dark:text-cool-grey-400 font-body">`)

	if ctx.Theme != nil && ctx.Theme.SupportContact != "" {
		b.WriteString(` · <a href="`)
		b.WriteString(layoutGetSupportHref(ctx.Theme.SupportContact))
		b.WriteString(`" class="hover:text-cool-grey-700 dark:hover:text-cool-grey-300 transition-colors">Contact Support</a>`)
	}

	b.WriteString(`</div>`)
	b.WriteString(`</footer>`)

	return template.HTML(b.String())
}

// layoutGetSupportHref returns the href for a support contact (mailto: or direct URL).
func layoutGetSupportHref(contact string) string {
	if len(contact) > 4 && contact[:4] == "http" {
		return contact
	}
	return "mailto:" + contact
}

// RenderScripts generates all utility scripts including:
// - Config data element (for basePath)
// - Logout script
// - Toast container and showToast()
// - Confirm modal and showConfirmModal()
// - Prompt modal and showPromptModal()
// - Auth script (fetch credentials)
//
// Usage in templates: {{ scripts . }}
func RenderScripts(ctx *TemplateContext, renderer *TemplateRenderer) template.HTML {
	var b strings.Builder

	// Config data element (always included)
	b.WriteString(`<div id="customer-layout-config" class="hidden" data-base-path="`)
	b.WriteString(template.HTMLEscapeString(ctx.BasePath))
	b.WriteString(`"></div>`)
	b.WriteString("\n")

	// Logout script (always included)
	b.WriteString(logoutScript)
	b.WriteString("\n")

	// Toast - check for override
	if override := renderPartialOverride(ctx, renderer, "toast"); override != "" {
		b.WriteString(string(override))
	} else {
		b.WriteString(toastHTML)
	}
	b.WriteString("\n")

	// Modal - check for override
	if override := renderPartialOverride(ctx, renderer, "modal"); override != "" {
		b.WriteString(string(override))
	} else {
		b.WriteString(confirmModalHTML)
		b.WriteString("\n")
		b.WriteString(promptModalHTML)
	}
	b.WriteString("\n")

	// Auth script (always included)
	b.WriteString(authScript)

	return template.HTML(b.String())
}
