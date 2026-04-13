package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRedirectOverviewWithAlert(t *testing.T) {
	h := &Handler{basePath: ""}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/installs/abc/reprovision", nil)

	h.redirectOverviewWithAlert(c, "abc", "success", "It worked")

	assert.Equal(t, http.StatusFound, w.Code)
	loc := w.Header().Get("Location")
	assert.Contains(t, loc, "/installs/abc/overview")
	assert.Contains(t, loc, "alert_type=success")
	assert.Contains(t, loc, "alert_msg=It+worked")
}

func TestRedirectOverviewWithAlert_SpecialChars(t *testing.T) {
	h := &Handler{basePath: ""}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/installs/abc/reprovision", nil)

	h.redirectOverviewWithAlert(c, "abc", "error", "Failed: something & bad")

	loc := w.Header().Get("Location")
	assert.Contains(t, loc, "alert_type=error")
	// URL-encoded ampersand
	assert.Contains(t, loc, "Failed%3A+something+%26+bad")
}

func TestInstallOverviewPage_NoInstall(t *testing.T) {
	h := &Handler{basePath: ""}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/installs/test/overview", nil)
	// No install set in context

	h.InstallOverviewPage(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
