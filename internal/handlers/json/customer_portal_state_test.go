package jsonhandlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/testutil"
)

func TestCustomerPortalState_ReturnsLayoutData(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		customer := testutil.NewTestCustomer()
		require.NoError(t, tx.Create(customer).Error)

		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: customer.ID, Name: "Acme Cloud", Subdomain: "acme"})
		require.NoError(t, tx.Create(org).Error)

		theme := testutil.NewTestTheme(testutil.ThemeOptions{OrgID: org.ID, PrimaryColor: "#123456"})
		theme.HeaderTitle = "Acme Portal"
		theme.LogoLightBase64 = "data:image/png;base64,light"
		theme.LogoDarkBase64 = "data:image/png;base64,dark"
		require.NoError(t, tx.Create(theme).Error)

		activeAccount := testutil.NewTestCustomerAccount(testutil.CustomerAccountOptions{
			OrgID:           org.ID,
			Name:            "Platform Team",
			CreatedByUserID: customer.ID,
		})
		otherAccount := testutil.NewTestCustomerAccount(testutil.CustomerAccountOptions{
			OrgID:           org.ID,
			Name:            "Security Team",
			CreatedByUserID: customer.ID,
		})
		require.NoError(t, tx.Create(activeAccount).Error)
		require.NoError(t, tx.Create(otherAccount).Error)

		require.NoError(t, tx.Create(testutil.NewTestCustomerAccountMember(testutil.CustomerAccountMemberOptions{
			AccountID: activeAccount.ID,
			UserID:    customer.ID,
			OrgID:     org.ID,
		})).Error)
		require.NoError(t, tx.Create(testutil.NewTestCustomerAccountMember(testutil.CustomerAccountMemberOptions{
			AccountID: otherAccount.ID,
			UserID:    customer.ID,
			OrgID:     org.ID,
		})).Error)

		install := testutil.NewTestInstall(testutil.InstallOptions{
			OrgID:  org.ID,
			UserID: customer.ID,
			Name:   "Payments Prod",
			Status: models.StatusActive,
		})
		install.InstallLinkID = nil
		install.NuonAppID = "app-payments"
		install.AppName = "Payments"
		install.CustomerAccountID = &activeAccount.ID
		install.Visibility = models.VisibilityAccount
		require.NoError(t, tx.Create(install).Error)

		publishedApp := &models.PublishedApp{
			OrgID:            org.ID,
			AppID:            "app-payments",
			Status:           models.AppStatusPublished,
			LogoLightBase64:  "data:image/png;base64,app-light",
			LogoDarkBase64:   "data:image/png;base64,app-dark",
			OverviewMarkdown: "# Payments\nDeploy the payments stack.",
		}
		require.NoError(t, tx.Create(publishedApp).Error)

		h := NewCustomerPortalHandler(tx, "http://localhost:8080", "localhost:8080", "http://localhost:8081")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/portal-api/state", nil)
		c.Set("subdomain", org.Subdomain)
		c.Request.AddCookie(&http.Cookie{Name: "active_account_id", Value: activeAccount.ID})
		c.Set("JWT_PAYLOAD", jwt.MapClaims{
			"user_id": customer.ID,
			"name":    customer.Name,
			"email":   customer.Email,
			"role":    string(customer.Role),
		})

		h.State(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response customerPortalStateResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Equal(t, "Acme Portal", response.Theme.HeaderTitle)
		assert.Equal(t, "Platform Team", response.ActiveAccount.Name)
		assert.Len(t, response.OtherAccounts, 1)
		assert.Equal(t, "Security Team", response.OtherAccounts[0].Name)
		assert.Len(t, response.Installs, 1)
		assert.Equal(t, "Payments Prod", response.Installs[0].Name)
		assert.Equal(t, "data:image/png;base64,app-light", response.Installs[0].AppLogoLight)
		assert.Len(t, response.Apps, 1)
		assert.Equal(t, "app-payments", response.Apps[0].AppID)
		assert.Equal(t, "Deploy the payments stack.", response.Apps[0].Summary)
		assert.Equal(t, "http://localhost:8080/admin/orgs/"+org.ID, response.Org.AdminURL)
	})
}

func TestCustomerPortalState_AllowsExplicitSubdomainQuery(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		customer := testutil.NewTestCustomer()
		require.NoError(t, tx.Create(customer).Error)

		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: customer.ID, Name: "Acme Cloud", Subdomain: "acme"})
		require.NoError(t, tx.Create(org).Error)

		h := NewCustomerPortalHandler(tx, "http://localhost:8080", "localhost:8080", "http://localhost:8081")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/portal-api/state?subdomain=acme", nil)
		c.Set("JWT_PAYLOAD", jwt.MapClaims{
			"user_id": customer.ID,
			"name":    customer.Name,
			"email":   customer.Email,
			"role":    string(customer.Role),
		})

		h.State(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response customerPortalStateResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Equal(t, org.ID, response.Org.ID)
		assert.Equal(t, org.Subdomain, response.Org.Subdomain)
	})
}
