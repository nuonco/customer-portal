package components

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderComponent(t *testing.T, component templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	err := component.Render(context.Background(), &buf)
	require.NoError(t, err)
	return buf.String()
}

func TestShimmerBar(t *testing.T) {
	html := renderComponent(t, ShimmerBar("h-4", "w-3/4"))
	assert.Contains(t, html, "shimmer")
	assert.Contains(t, html, "h-4")
	assert.Contains(t, html, "w-3/4")
}

func TestShimmerLines(t *testing.T) {
	html := renderComponent(t, ShimmerLines(3))
	count := strings.Count(html, "shimmer")
	assert.Equal(t, 3, count, "should render 3 shimmer bars")
	assert.Contains(t, html, "w-3/4", "even-indexed bars use w-3/4")
	assert.Contains(t, html, "w-1/2", "odd-indexed bars use w-1/2")
}

func TestShimmerLinesZero(t *testing.T) {
	html := renderComponent(t, ShimmerLines(0))
	assert.NotContains(t, html, "shimmer", "zero lines should render no shimmer bars")
}
