package templates

import (
	"bytes"
	"testing"
)

func TestNewRenderer(t *testing.T) {
	renderer, err := NewRenderer(false)
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}

	names := renderer.TemplateNames()
	if len(names) == 0 {
		t.Fatal("Expected templates to be loaded, got 0")
	}

	t.Logf("Loaded %d templates", len(names))
	for _, name := range names {
		t.Logf("  - %s", name)
	}
}

func TestRenderLoginPage(t *testing.T) {
	renderer, err := NewRenderer(false)
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}

	data := LoginPageData{
		Title:      "Test Login",
		ButtonText: "Sign In",
		BasePath:   "/admin",
		Theme:      &ThemeData{PrimaryColor: "#2563EB"},
	}

	var buf bytes.Buffer
	err = renderer.RenderPartial(&buf, "vendor-login", data)
	if err != nil {
		t.Fatalf("Failed to render vendor-login: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("Expected non-empty output")
	}

	t.Logf("Rendered %d bytes", buf.Len())
}

func TestRenderErrorPage(t *testing.T) {
	renderer, err := NewRenderer(false)
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}

	data := ErrorPageData{
		Error:    "Test error message",
		BasePath: "/admin",
		Theme:    &ThemeData{PrimaryColor: "#2563EB"},
	}

	var buf bytes.Buffer
	err = renderer.RenderPartial(&buf, "vendor-error", data)
	if err != nil {
		t.Fatalf("Failed to render vendor-error: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("Expected non-empty output")
	}

	t.Logf("Rendered %d bytes", buf.Len())
}
