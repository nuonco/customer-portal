package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/testutil"
)

func TestCustomerAppsPage_EmptyApps(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		_, org, err := testutil.SeedMinimalData(tx)
		require.NoError(t, err)

		logger := zap.NewNop()
		h := NewHandler(tx, nil, nil, "http://localhost:8080", "https://api.nuon.co", "", "", "localhost:8080", "nuon.co", logger)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/apps", nil)
		c.Set("subdomain", org.Subdomain)

		h.CustomerAppsPage(c)

		assert.Equal(t, http.StatusOK, w.Code, "should render page instead of redirecting")
		body := w.Body.String()
		assert.Contains(t, body, "No Apps Available", "should contain empty state title")
		assert.Contains(t, body, "No applications are currently available", "should contain empty state message")
	})
}

func TestCustomerAppsPage_SingleApp_FullWidthCard(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		_, org, err := testutil.SeedMinimalData(tx)
		require.NoError(t, err)

		pa := &models.PublishedApp{
			OrgID: org.ID,
			AppID: "app-test-12345",
		}
		require.NoError(t, tx.Create(pa).Error)

		logger := zap.NewNop()
		h := NewHandler(tx, nil, nil, "http://localhost:8080", "https://api.nuon.co", "", "", "localhost:8080", "nuon.co", logger)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/apps", nil)
		c.Set("subdomain", org.Subdomain)

		h.CustomerAppsPage(c)

		assert.Equal(t, http.StatusOK, w.Code, "should render page with single app")
		body := w.Body.String()
		assert.NotContains(t, body, "No Apps Available", "should not show empty state")
		assert.Contains(t, body, "card-lg", "single app should render full-width card")
		assert.NotContains(t, body, "card-base", "single app should not render grid card")
	})
}

func TestCustomerAppsPage_MultipleApps_Grid(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		_, org, err := testutil.SeedMinimalData(tx)
		require.NoError(t, err)

		for _, appID := range []string{"app-test-1", "app-test-2"} {
			require.NoError(t, tx.Create(&models.PublishedApp{
				OrgID: org.ID,
				AppID: appID,
			}).Error)
		}

		logger := zap.NewNop()
		h := NewHandler(tx, nil, nil, "http://localhost:8080", "https://api.nuon.co", "", "", "localhost:8080", "nuon.co", logger)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/apps", nil)
		c.Set("subdomain", org.Subdomain)

		h.CustomerAppsPage(c)

		assert.Equal(t, http.StatusOK, w.Code, "should render page with multiple apps")
		body := w.Body.String()
		assert.NotContains(t, body, "No Apps Available", "should not show empty state")
		assert.Contains(t, body, "card-base", "multiple apps should render grid cards")
	})
}
