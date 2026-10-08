package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/internal/testutil"
)

func TestDeleteOrg_ArchivesOrg(t *testing.T) {
	testutil.TestTransaction(t, func(db *gorm.DB) {
		vendor := testutil.NewTestVendor()
		require.NoError(t, db.Create(vendor).Error)
		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID})
		require.NoError(t, db.Create(org).Error)

		h := &Handler{db: db, logger: zap.NewNop(), basePath: "/admin"}

		c, w := testutil.NewTestContextWithRequest(http.MethodDelete, "/admin/orgs/"+org.ID, nil)
		c.Set("org", org)

		h.DeleteOrg(c)
		assert.Equal(t, http.StatusOK, w.Code)

		// Verify soft-deleted (not found without Unscoped)
		var count int64
		db.Model(&models.NuonOrg{}).Where("id = ?", org.ID).Count(&count)
		assert.Equal(t, int64(0), count, "org should not be found without Unscoped")

		// Verify still exists with Unscoped
		var archived models.NuonOrg
		err := db.Unscoped().Where("id = ?", org.ID).First(&archived).Error
		require.NoError(t, err)
		assert.True(t, archived.DeletedAt.Valid, "deleted_at should be set")

		// Verify API token was cleared
		assert.Empty(t, archived.APIToken, "api_token should be cleared")
	})
}

func TestDeleteOrg_HTMXRedirect(t *testing.T) {
	testutil.TestTransaction(t, func(db *gorm.DB) {
		vendor := testutil.NewTestVendor()
		require.NoError(t, db.Create(vendor).Error)
		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID})
		require.NoError(t, db.Create(org).Error)

		h := &Handler{db: db, logger: zap.NewNop(), basePath: "/admin"}

		c, w := testutil.NewTestContextWithRequest(http.MethodDelete, "/admin/orgs/"+org.ID, nil)
		c.Request.Header.Set("HX-Request", "true")
		c.Set("org", org)

		h.DeleteOrg(c)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "/admin/orgs", w.Header().Get("HX-Redirect"))
	})
}

func TestDeleteOrg_ArchivedOrgNotInList(t *testing.T) {
	testutil.TestTransaction(t, func(db *gorm.DB) {
		vendor := testutil.NewTestVendor()
		require.NoError(t, db.Create(vendor).Error)
		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID})
		require.NoError(t, db.Create(org).Error)

		h := &Handler{db: db, logger: zap.NewNop(), basePath: "/admin"}

		// Archive it
		c, _ := testutil.NewTestContextWithRequest(http.MethodDelete, "/admin/orgs/"+org.ID, nil)
		c.Set("org", org)
		h.DeleteOrg(c)

		// Query active orgs - archived should not appear
		var orgs []models.NuonOrg
		db.Where("user_id = ?", vendor.ID).Find(&orgs)
		assert.Empty(t, orgs, "archived org should not appear in normal queries")
	})
}

func TestRestoreOrg_Success(t *testing.T) {
	testutil.TestTransaction(t, func(db *gorm.DB) {
		vendor := testutil.NewTestVendor()
		require.NoError(t, db.Create(vendor).Error)
		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID})
		require.NoError(t, db.Create(org).Error)

		h := &Handler{db: db, logger: zap.NewNop(), basePath: "/admin"}

		// Archive it first
		c, _ := testutil.NewTestContextWithRequest(http.MethodDelete, "/admin/orgs/"+org.ID, nil)
		c.Set("org", org)
		h.DeleteOrg(c)

		// Restore it
		body, _ := json.Marshal(map[string]string{"api_token": "new-token-123"})
		c2, w2 := testutil.NewTestContextWithRequest(http.MethodPost, "/admin/orgs/"+org.ID+"/restore", body)
		c2.Request.Header.Set("Content-Type", "application/json")
		c2.Params = gin.Params{{Key: "org_id", Value: org.ID}}
		c2.Set("user", vendor)

		h.RestoreOrg(c2)
		assert.Equal(t, http.StatusOK, w2.Code)

		// Verify restored
		var restored models.NuonOrg
		err := db.Where("id = ?", org.ID).First(&restored).Error
		require.NoError(t, err, "org should be found after restore")
		assert.False(t, restored.DeletedAt.Valid, "deleted_at should be nil")
		assert.Equal(t, "new-token-123", restored.APIToken, "new API token should be set")
	})
}

func TestNuonAPIURLForOrg_ReturnsOrgURLWhenSet(t *testing.T) {
	h := &Handler{nuonAPIURL: "https://api.nuon.co"}

	org := &models.NuonOrg{APIURL: "https://custom.api.example.com"}
	assert.Equal(t, "https://custom.api.example.com", h.nuonAPIURLForOrg(org))
}

func TestNuonAPIURLForOrg_ReturnsGlobalWhenOrgURLEmpty(t *testing.T) {
	h := &Handler{nuonAPIURL: "https://api.nuon.co"}

	org := &models.NuonOrg{}
	assert.Equal(t, "https://api.nuon.co", h.nuonAPIURLForOrg(org))
}

func TestNuonAPIURLForOrg_ReturnsGlobalWhenOrgNil(t *testing.T) {
	h := &Handler{nuonAPIURL: "https://api.nuon.co"}
	assert.Equal(t, "https://api.nuon.co", h.nuonAPIURLForOrg(nil))
}

func TestCreateOrg_StoresAPIURL(t *testing.T) {
	testutil.TestTransaction(t, func(db *gorm.DB) {
		vendor := testutil.NewTestVendor()
		require.NoError(t, db.Create(vendor).Error)

		// Create an org directly with APIURL to verify the field persists
		org := &models.NuonOrg{
			UserID:    vendor.ID,
			NuonOrgID: "org_test_apiurl",
			APIToken:  "tok_test",
			APIURL:    "https://staging.api.nuon.co",
			Name:      "Test Org",
			Subdomain: "test-apiurl",
		}
		require.NoError(t, db.Create(org).Error)

		var loaded models.NuonOrg
		require.NoError(t, db.Where("id = ?", org.ID).First(&loaded).Error)
		assert.Equal(t, "https://staging.api.nuon.co", loaded.APIURL)
	})
}

func TestRestoreOrg_RequiresAPIToken(t *testing.T) {
	testutil.TestTransaction(t, func(db *gorm.DB) {
		vendor := testutil.NewTestVendor()
		require.NoError(t, db.Create(vendor).Error)
		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID})
		require.NoError(t, db.Create(org).Error)

		h := &Handler{db: db, logger: zap.NewNop(), basePath: "/admin"}

		// Archive it
		c, _ := testutil.NewTestContextWithRequest(http.MethodDelete, "/admin/orgs/"+org.ID, nil)
		c.Set("org", org)
		h.DeleteOrg(c)

		// Try restore without token
		body, _ := json.Marshal(map[string]string{})
		c2, w2 := testutil.NewTestContextWithRequest(http.MethodPost, "/admin/orgs/"+org.ID+"/restore", body)
		c2.Request.Header.Set("Content-Type", "application/json")
		c2.Params = gin.Params{{Key: "org_id", Value: org.ID}}
		c2.Set("user", vendor)

		h.RestoreOrg(c2)
		assert.Equal(t, http.StatusBadRequest, w2.Code)
	})
}
