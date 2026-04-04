package highlight

import (
	"bytes"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

var (
	formatter = chromahtml.New(chromahtml.WithClasses(true))
	style     = styles.Fallback
)

// Highlight returns syntax-highlighted HTML for the given code and language.
// If lang is empty, it auto-detects. Falls back to plain text on any error.
func Highlight(code, lang string) string {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)
	iter, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}
	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iter); err != nil {
		return code
	}
	return buf.String()
}
