package markdown

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// ToHTML converts markdown to safe HTML
// Features enabled:
// - CommonMark spec compliance
// - GitHub Flavored Markdown (tables, strikethrough, task lists)
// - Safe rendering with raw HTML support for custom README content
func ToHTML(markdown string) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,     // GitHub Flavored Markdown
			extension.Linkify, // Auto-link URLs
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // Add IDs to headings
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // Allow raw HTML in markdown (readmes may have custom HTML)
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}
