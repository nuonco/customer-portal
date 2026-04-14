package utils

import (
	"fmt"
	"net/url"
	"strings"
)

// URLBuilder constructs URLs with query parameters.
// All methods return the builder for chaining.
// Use Build() to get the final URL string.
type URLBuilder struct {
	path   string
	params []param
}

type param struct {
	key   string
	value string
}

// NewURL creates a URLBuilder from path segments joined with "/".
func NewURL(segments ...string) *URLBuilder {
	return &URLBuilder{path: strings.Join(segments, "/")}
}

// Set adds a string query parameter. Skipped if value is empty.
func (b *URLBuilder) Set(key, value string) *URLBuilder {
	if value != "" {
		b.params = append(b.params, param{key, value})
	}
	return b
}

// SetInt adds an integer query parameter. Skipped if value < 0.
func (b *URLBuilder) SetInt(key string, value int) *URLBuilder {
	if value >= 0 {
		b.params = append(b.params, param{key, fmt.Sprintf("%d", value)})
	}
	return b
}

// SetBool adds a boolean query parameter (key=true). Skipped if false.
func (b *URLBuilder) SetBool(key string, value bool) *URLBuilder {
	if value {
		b.params = append(b.params, param{key, "true"})
	}
	return b
}

// SetEncoded adds a query parameter with URL-encoded value. Skipped if value is empty.
func (b *URLBuilder) SetEncoded(key, value string) *URLBuilder {
	if value != "" {
		b.params = append(b.params, param{key, url.QueryEscape(value)})
	}
	return b
}

// Build returns the final URL string.
func (b *URLBuilder) Build() string {
	if len(b.params) == 0 {
		return b.path
	}
	var sb strings.Builder
	sb.WriteString(b.path)
	for i, p := range b.params {
		if i == 0 {
			sb.WriteByte('?')
		} else {
			sb.WriteByte('&')
		}
		sb.WriteString(p.key)
		sb.WriteByte('=')
		sb.WriteString(p.value)
	}
	return sb.String()
}
