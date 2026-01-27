package overrides

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

func TestRenderHead(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *TemplateContext
		contains []string
	}{
		{
			name: "basic head with title and CSS",
			ctx: &TemplateContext{
				Title:   "Test Page",
				CSSPath: "/static/customer.css",
			},
			contains: []string{
				"<head>",
				"</head>",
				"<title>Test Page</title>",
				`href="/static/customer.css"`,
				"prefers-color-scheme: dark",
				"fonts.googleapis.com",
				"htmx.org",
			},
		},
		{
			name: "head with custom CSS path",
			ctx: &TemplateContext{
				Title:         "Custom CSS",
				CSSPath:       "/static/main.css",
				CustomCSSPath: "/custom/css/workspace.css",
			},
			contains: []string{
				`href="/static/main.css"`,
				`href="/custom/css/workspace.css"`,
			},
		},
		{
			name: "head with theme colors",
			ctx: &TemplateContext{
				Title:   "Themed",
				CSSPath: "/css/main.css",
				Theme: &ThemeData{
					PrimaryColor:     "#FF0000",
					PrimaryColorDark: "#CC0000",
				},
			},
			contains: []string{
				"--theme-primary: #FF0000",
				"--theme-primary-hover: #CC0000",
			},
		},
		{
			name: "head with Google font",
			ctx: &TemplateContext{
				Title:   "Custom Font",
				CSSPath: "/css/main.css",
				Theme: &ThemeData{
					HeadingFont: "Roboto",
				},
			},
			contains: []string{
				"family=Roboto",
				"--font-heading: 'Roboto'",
			},
		},
		{
			name: "head with custom base64 font",
			ctx: &TemplateContext{
				Title:   "Base64 Font",
				CSSPath: "/css/main.css",
				Theme: &ThemeData{
					HeadingFontBase64: "data:font/woff2;base64,ABC",
				},
			},
			contains: []string{
				"@font-face",
				"CustomHeading",
				"--font-heading: 'CustomHeading'",
			},
		},
		{
			name: "escapes title",
			ctx: &TemplateContext{
				Title:   "<script>alert('xss')</script>",
				CSSPath: "/css/main.css",
			},
			contains: []string{
				"&lt;script&gt;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html := RenderHead(tt.ctx)
			htmlStr := string(html)

			for _, want := range tt.contains {
				if !strings.Contains(htmlStr, want) {
					t.Errorf("RenderHead() missing %q\nGot: %s", want, htmlStr)
				}
			}
		})
	}
}

func TestRenderBodyAttrs(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *TemplateContext
		contains []string
	}{
		{
			name: "basic body attrs",
			ctx:  &TemplateContext{},
			contains: []string{
				"bg-cool-grey-50",
				"dark:bg-dark-grey-950",
				"dark:text-white",
			},
		},
		{
			name: "with radius class",
			ctx: &TemplateContext{
				Theme: &ThemeData{
					RadiusClass: "radius-rounded",
				},
			},
			contains: []string{
				"radius-rounded",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := RenderBodyAttrs(tt.ctx)
			attrStr := string(attrs)

			for _, want := range tt.contains {
				if !strings.Contains(attrStr, want) {
					t.Errorf("RenderBodyAttrs() missing %q\nGot: %s", want, attrStr)
				}
			}

			// Should start with class=
			if !strings.HasPrefix(attrStr, `class="`) {
				t.Errorf("RenderBodyAttrs() should start with class=, got: %s", attrStr)
			}
		})
	}
}

func TestRenderHeader(t *testing.T) {
	tests := []struct {
		name        string
		ctx         *TemplateContext
		contains    []string
		notContains []string
	}{
		{
			name: "header with user",
			ctx: &TemplateContext{
				User: &UserData{Email: "test@example.com"},
			},
			contains: []string{
				"<header",
				"test@example.com",
				"logout()",
				"Logout",
			},
		},
		{
			name: "header with logo",
			ctx: &TemplateContext{
				User: &UserData{Email: "user@test.com"},
				Theme: &ThemeData{
					LogoBase64: "data:image/png;base64,ABC123",
				},
			},
			contains: []string{
				`src="data:image/png;base64,ABC123"`,
				`alt="Logo"`,
			},
		},
		{
			name:     "header without user",
			ctx:      &TemplateContext{},
			contains: []string{"<header"},
			notContains: []string{
				"logout()",
				"Logout",
			},
		},
		{
			name: "escapes user email",
			ctx: &TemplateContext{
				User: &UserData{Email: "<script>bad</script>@test.com"},
			},
			contains: []string{
				"&lt;script&gt;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html := RenderHeader(tt.ctx, nil)
			htmlStr := string(html)

			for _, want := range tt.contains {
				if !strings.Contains(htmlStr, want) {
					t.Errorf("RenderHeader() missing %q\nGot: %s", want, htmlStr)
				}
			}

			for _, notWant := range tt.notContains {
				if strings.Contains(htmlStr, notWant) {
					t.Errorf("RenderHeader() should not contain %q\nGot: %s", notWant, htmlStr)
				}
			}
		})
	}
}

func TestRenderFooter(t *testing.T) {
	tests := []struct {
		name        string
		ctx         *TemplateContext
		contains    []string
		notContains []string
	}{
		{
			name:     "basic footer",
			ctx:      &TemplateContext{},
			contains: []string{"<footer", "Powered by Nuon", "</footer>"},
		},
		{
			name: "footer with email support",
			ctx: &TemplateContext{
				Theme: &ThemeData{
					SupportContact: "support@example.com",
				},
			},
			contains: []string{
				"Contact Support",
				`href="mailto:support@example.com"`,
			},
		},
		{
			name: "footer with URL support",
			ctx: &TemplateContext{
				Theme: &ThemeData{
					SupportContact: "https://support.example.com",
				},
			},
			contains: []string{
				"Contact Support",
				`href="https://support.example.com"`,
			},
			notContains: []string{
				"mailto:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html := RenderFooter(tt.ctx, nil)
			htmlStr := string(html)

			for _, want := range tt.contains {
				if !strings.Contains(htmlStr, want) {
					t.Errorf("RenderFooter() missing %q\nGot: %s", want, htmlStr)
				}
			}

			for _, notWant := range tt.notContains {
				if strings.Contains(htmlStr, notWant) {
					t.Errorf("RenderFooter() should not contain %q\nGot: %s", notWant, htmlStr)
				}
			}
		})
	}
}

func TestRenderScripts(t *testing.T) {
	ctx := &TemplateContext{
		BasePath: "/app",
	}

	html := RenderScripts(ctx, nil)
	htmlStr := string(html)

	expected := []string{
		`data-base-path="/app"`,
		"customer-layout-config",
		"function logout()",
		"window.showToast",
		"toast-container",
		"window.showConfirmModal",
		"confirm-modal",
		"window.showPromptModal",
		"prompt-modal",
		"credentials",
	}

	for _, want := range expected {
		if !strings.Contains(htmlStr, want) {
			t.Errorf("RenderScripts() missing %q", want)
		}
	}
}

func TestRenderPreviewBanner(t *testing.T) {
	html := RenderPreviewBanner()
	htmlStr := string(html)

	expected := []string{
		"preview-banner",
		"preview",
		"closePreviewBanner",
	}

	for _, want := range expected {
		if !strings.Contains(htmlStr, want) {
			t.Errorf("RenderPreviewBanner() missing %q", want)
		}
	}
}

func TestLayoutComponentsInTemplate(t *testing.T) {
	tests := []struct {
		name     string
		tmplStr  string
		ctx      *TemplateContext
		contains []string
	}{
		{
			name:    "head in template",
			tmplStr: `<!DOCTYPE html><html>{{ head . }}</html>`,
			ctx: &TemplateContext{
				Title:   "Test",
				CSSPath: "/css/app.css",
			},
			contains: []string{"<head>", "<title>Test</title>", "</head>"},
		},
		{
			name:    "bodyAttrs in template",
			tmplStr: `<body {{ bodyAttrs . }}>content</body>`,
			ctx: &TemplateContext{
				Theme: &ThemeData{
					RadiusClass: "radius-sharp",
				},
			},
			contains: []string{`class="`, "radius-sharp"},
		},
		{
			name:    "header in template",
			tmplStr: `{{ header . }}`,
			ctx: &TemplateContext{
				User: &UserData{Email: "user@test.com"},
			},
			contains: []string{"<header", "user@test.com"},
		},
		{
			name:     "footer in template",
			tmplStr:  `{{ footer . }}`,
			ctx:      &TemplateContext{},
			contains: []string{"<footer", "Powered by Nuon"},
		},
		{
			name:    "scripts in template",
			tmplStr: `{{ scripts . }}`,
			ctx: &TemplateContext{
				BasePath: "/dashboard",
			},
			contains: []string{"showToast", "showConfirmModal"},
		},
		{
			name:     "previewBanner in template",
			tmplStr:  `{{ previewBanner }}`,
			ctx:      &TemplateContext{},
			contains: []string{"preview-banner"},
		},
		{
			name: "full layout template",
			tmplStr: `<!DOCTYPE html>
<html lang="en">
{{ head . }}
<body {{ bodyAttrs . }}>
{{ previewBanner }}
{{ header . }}
<main>Content</main>
{{ footer . }}
{{ scripts . }}
</body>
</html>`,
			ctx: &TemplateContext{
				Title:    "Full Layout",
				CSSPath:  "/css/main.css",
				BasePath: "/app",
				User:     &UserData{Email: "test@example.com"},
				Theme: &ThemeData{
					PrimaryColor:   "#123456",
					RadiusClass:    "radius-rounded",
					SupportContact: "help@example.com",
				},
			},
			contains: []string{
				"<!DOCTYPE html>",
				"<title>Full Layout</title>",
				"--theme-primary: #123456",
				"radius-rounded",
				"test@example.com",
				"Powered by Nuon",
				"mailto:help@example.com",
				"showToast",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("test").Funcs(getBaseFuncMap()).Parse(tt.tmplStr)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, tt.ctx); err != nil {
				t.Fatalf("Failed to execute template: %v", err)
			}

			result := buf.String()
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("Template result missing %q\nGot: %s", want, result)
				}
			}
		})
	}
}

func TestBuildGoogleFontsLink(t *testing.T) {
	tests := []struct {
		name     string
		theme    *ThemeData
		contains string
		empty    bool
	}{
		{
			name:  "nil theme",
			theme: nil,
			empty: true,
		},
		{
			name:  "no fonts",
			theme: &ThemeData{},
			empty: true,
		},
		{
			name: "heading font only",
			theme: &ThemeData{
				HeadingFont: "Roboto",
			},
			contains: "family=Roboto",
		},
		{
			name: "body font only",
			theme: &ThemeData{
				BodyFont: "Open Sans",
			},
			contains: "family=Open Sans",
		},
		{
			name: "both fonts different",
			theme: &ThemeData{
				HeadingFont: "Roboto",
				BodyFont:    "Open Sans",
			},
			contains: "family=Roboto",
		},
		{
			name: "base64 font skips Google",
			theme: &ThemeData{
				HeadingFont:       "Roboto",
				HeadingFontBase64: "data:font/woff2;base64,ABC",
			},
			empty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildGoogleFontsLink(tt.theme)

			if tt.empty {
				if result != "" {
					t.Errorf("buildGoogleFontsLink() = %q, want empty", result)
				}
				return
			}

			if !strings.Contains(result, tt.contains) {
				t.Errorf("buildGoogleFontsLink() = %q, want to contain %q", result, tt.contains)
			}
		})
	}
}

func TestBuildThemeCSS(t *testing.T) {
	tests := []struct {
		name     string
		theme    *ThemeData
		contains []string
		empty    bool
	}{
		{
			name:  "nil theme",
			theme: nil,
			empty: true,
		},
		{
			name:  "empty theme",
			theme: &ThemeData{},
			empty: true,
		},
		{
			name: "primary color",
			theme: &ThemeData{
				PrimaryColor:     "#FF0000",
				PrimaryColorDark: "#CC0000",
			},
			contains: []string{
				"--theme-primary: #FF0000",
				"--theme-primary-hover: #CC0000",
			},
		},
		{
			name: "secondary color",
			theme: &ThemeData{
				SecondaryColor:     "#00FF00",
				SecondaryColorDark: "#00CC00",
			},
			contains: []string{
				"--theme-secondary: #00FF00",
				"--theme-secondary-hover: #00CC00",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildThemeCSS(tt.theme)

			if tt.empty {
				if result != "" {
					t.Errorf("buildThemeCSS() = %q, want empty", result)
				}
				return
			}

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("buildThemeCSS() missing %q\nGot: %s", want, result)
				}
			}
		})
	}
}
