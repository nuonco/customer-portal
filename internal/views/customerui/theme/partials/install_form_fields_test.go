package partials

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

func TestRegionSelector_AWS(t *testing.T) {
	html := renderComponent(t, RegionSelector("aws-eks"))
	assert.Contains(t, html, `name="region"`)
	assert.Contains(t, html, "us-east-1")
	assert.Contains(t, html, "eu-central-1")
	assert.Contains(t, html, "ap-northeast-1")
	assert.NotContains(t, html, `name="location"`)
}

func TestRegionSelector_Azure(t *testing.T) {
	html := renderComponent(t, RegionSelector("azure-aks"))
	assert.Contains(t, html, `name="location"`)
	assert.Contains(t, html, "eastus")
	assert.Contains(t, html, "westeurope")
	assert.Contains(t, html, "japaneast")
	assert.NotContains(t, html, `name="region"`)
}

func TestRegionSelector_GCP(t *testing.T) {
	html := renderComponent(t, RegionSelector("gcp"))
	assert.Equal(t, "", strings.TrimSpace(html))
}

func TestRegionSelector_Default(t *testing.T) {
	html := renderComponent(t, RegionSelector(""))
	assert.Contains(t, html, `name="region"`)
	assert.Contains(t, html, "us-east-1")
}

func TestInstallFormFields_GCPHidesRegion(t *testing.T) {
	config := InstallFormConfig{Platform: "gcp"}
	html := renderComponent(t, InstallFormFields(config))
	assert.NotContains(t, html, "Deployment Region")
}

func TestInstallFormFields_AWSShowsRegion(t *testing.T) {
	config := InstallFormConfig{Platform: "aws-eks"}
	html := renderComponent(t, InstallFormFields(config))
	assert.Contains(t, html, "Deployment Region")
	assert.Contains(t, html, `name="region"`)
}
