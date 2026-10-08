package handlers

import (
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

func TestForgetApp_SoftDeletes(t *testing.T) {
	testutil.TestTransaction(t, func(db *gorm.DB) {
		vendor := testutil.NewTestVendor()
		require.NoError(t, db.Create(vendor).Error)
		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID})
		require.NoError(t, db.Create(org).Error)

		app := models.PublishedApp{OrgID: org.ID, AppID: "app_orphan", Status: "published"}
		require.NoError(t, db.Create(&app).Error)

		h := &Handler{db: db, logger: zap.NewNop()}

		c, w := testutil.NewTestContextWithRequest(http.MethodDelete, "/admin/orgs/"+org.ID+"/apps/app_orphan/forget", nil)
		c.Params = gin.Params{{Key: "app_id", Value: "app_orphan"}}
		c.Set("org", org)

		h.ForgetApp(c)
		assert.Equal(t, http.StatusOK, w.Code)

		// Verify soft-deleted (not found without Unscoped)
		var count int64
		db.Model(&models.PublishedApp{}).Where("org_id = ? AND app_id = ?", org.ID, "app_orphan").Count(&count)
		assert.Equal(t, int64(0), count)

		// Verify still exists with Unscoped
		db.Unscoped().Model(&models.PublishedApp{}).Where("org_id = ? AND app_id = ?", org.ID, "app_orphan").Count(&count)
		assert.Equal(t, int64(1), count)
	})
}

func TestForgetApp_NotFound(t *testing.T) {
	testutil.TestTransaction(t, func(db *gorm.DB) {
		vendor := testutil.NewTestVendor()
		require.NoError(t, db.Create(vendor).Error)
		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID})
		require.NoError(t, db.Create(org).Error)

		h := &Handler{db: db, logger: zap.NewNop()}

		c, w := testutil.NewTestContextWithRequest(http.MethodDelete, "/admin/orgs/"+org.ID+"/apps/nonexistent/forget", nil)
		c.Params = gin.Params{{Key: "app_id", Value: "nonexistent"}}
		c.Set("org", org)

		h.ForgetApp(c)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
