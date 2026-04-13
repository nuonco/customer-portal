package pages

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildOverviewURL_Defaults(t *testing.T) {
	url := buildOverviewURL(overviewURLParams{
		BasePath:  "",
		InstallID: "abc",
	})
	assert.Equal(t, "/installs/abc/overview", url)
}

func TestBuildOverviewURL_WithTab(t *testing.T) {
	url := buildOverviewURL(overviewURLParams{
		BasePath:  "",
		InstallID: "abc",
		Tab:       "inputs",
	})
	assert.Equal(t, "/installs/abc/overview?tab=inputs", url)
}

func TestBuildOverviewURL_DefaultTabOmitted(t *testing.T) {
	url := buildOverviewURL(overviewURLParams{
		BasePath:  "",
		InstallID: "abc",
		Tab:       "history",
	})
	// "history" is the default tab, so it should be omitted
	assert.Equal(t, "/installs/abc/overview", url)
}

func TestBuildOverviewURL_WithPartial(t *testing.T) {
	url := buildOverviewURL(overviewURLParams{
		BasePath:  "",
		InstallID: "abc",
		Tab:       "inputs",
		Partial:   "content",
	})
	assert.Equal(t, "/installs/abc/overview?tab=inputs&partial=content", url)
}

func TestBuildOverviewURL_WithAlert(t *testing.T) {
	url := buildOverviewURL(overviewURLParams{
		BasePath:  "",
		InstallID: "abc",
		AlertType: "success",
		AlertMsg:  "It worked",
	})
	assert.Equal(t, "/installs/abc/overview?alert_type=success&alert_msg=It+worked", url)
}

func TestBuildOverviewURL_AlertRequiresBothFields(t *testing.T) {
	// Alert with only type and no message should be omitted
	url := buildOverviewURL(overviewURLParams{
		BasePath:  "",
		InstallID: "abc",
		AlertType: "success",
	})
	assert.Equal(t, "/installs/abc/overview", url)
}

func TestBuildOverviewURL_WithBasePath(t *testing.T) {
	url := buildOverviewURL(overviewURLParams{
		BasePath:  "/portal",
		InstallID: "abc",
		Tab:       "inputs",
	})
	assert.Equal(t, "/portal/installs/abc/overview?tab=inputs", url)
}

func TestOverviewWithPartial(t *testing.T) {
	p := overviewURLParams{
		BasePath:  "",
		InstallID: "abc",
		Tab:       "inputs",
	}
	url := overviewWithPartial(p, "content")
	assert.Equal(t, "/installs/abc/overview?tab=inputs&partial=content", url)
}

func TestBuildOverviewURL_WithWorkflowID(t *testing.T) {
	url := buildOverviewURL(overviewURLParams{
		BasePath:   "",
		InstallID:  "abc",
		WorkflowID: "wf-123",
	})
	assert.Equal(t, "/installs/abc/overview/workflows/wf-123", url)
}

func TestBuildOverviewURL_WorkflowWithExpanded(t *testing.T) {
	url := buildOverviewURL(overviewURLParams{
		BasePath:   "",
		InstallID:  "abc",
		WorkflowID: "wf-123",
		Expanded:   true,
	})
	assert.Equal(t, "/installs/abc/overview/workflows/wf-123?expanded=true", url)
}

func TestBuildOverviewURL_WorkflowWithPartial(t *testing.T) {
	url := buildOverviewURL(overviewURLParams{
		BasePath:   "",
		InstallID:  "abc",
		WorkflowID: "wf-123",
		Partial:    "panel",
	})
	assert.Equal(t, "/installs/abc/overview/workflows/wf-123?partial=panel", url)
}

func TestBuildOverviewURL_WorkflowWithAlert(t *testing.T) {
	url := buildOverviewURL(overviewURLParams{
		BasePath:   "",
		InstallID:  "abc",
		WorkflowID: "wf-123",
		AlertType:  "success",
		AlertMsg:   "Approved",
	})
	assert.Equal(t, "/installs/abc/overview/workflows/wf-123?alert_type=success&alert_msg=Approved", url)
}
