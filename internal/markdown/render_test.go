package markdown

import (
	"strings"
	"testing"
)

func TestRender_SyntaxHighlighting(t *testing.T) {
	src := "```go\nfunc main() {}\n```\n"
	html, err := Render([]byte(src))
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if !strings.Contains(html, `class="chroma"`) {
		t.Errorf("expected chroma class in output, got: %s", html)
	}
	if !strings.Contains(html, "<span") {
		t.Errorf("expected token spans in output, got: %s", html)
	}
}

func TestRender_PlainMarkdown(t *testing.T) {
	src := "# Hello\n\nSome text.\n"
	html, err := Render([]byte(src))
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if !strings.Contains(html, "<h1>Hello</h1>") {
		t.Errorf("expected h1 in output, got: %s", html)
	}
}
