package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/internal/testutil"
)

func TestUpdateAppOrder_SortOrderPersists(t *testing.T) {
	testutil.TestTransaction(t, func(db *gorm.DB) {
		// Create test org
		vendor := testutil.NewTestVendor()
		require.NoError(t, db.Create(vendor).Error)
		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID})
		require.NoError(t, db.Create(org).Error)

		// Create published apps in initial order
		apps := []models.PublishedApp{
			{OrgID: org.ID, AppID: "app_aaa", SortOrder: 0, Status: "published"},
			{OrgID: org.ID, AppID: "app_bbb", SortOrder: 1, Status: "published"},
			{OrgID: org.ID, AppID: "app_ccc", SortOrder: 2, Status: "published"},
		}
		for i := range apps {
			require.NoError(t, db.Create(&apps[i]).Error)
		}

		// Set up handler with test db
		h := &Handler{db: db, logger: zap.NewNop()}

		// Build request: reverse the order
		body := map[string]interface{}{
			"app_ids": []string{"app_ccc", "app_bbb", "app_aaa"},
			"app_statuses": map[string]string{
				"app_ccc": "published",
				"app_bbb": "published",
				"app_aaa": "published",
			},
		}
		bodyBytes, _ := json.Marshal(body)
		c, w := testutil.NewTestContextWithRequest(http.MethodPut, "/admin/orgs/"+org.ID+"/apps/order", bodyBytes)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("org", org)

		h.UpdateAppOrder(c)
		assert.Equal(t, http.StatusOK, w.Code)

		// Verify sort order persisted
		var results []models.PublishedApp
		require.NoError(t, db.Where("org_id = ?", org.ID).Order("sort_order asc").Find(&results).Error)
		require.Len(t, results, 3)

		assert.Equal(t, "app_ccc", results[0].AppID)
		assert.Equal(t, 0, results[0].SortOrder)
		assert.Equal(t, "app_bbb", results[1].AppID)
		assert.Equal(t, 1, results[1].SortOrder)
		assert.Equal(t, "app_aaa", results[2].AppID)
		assert.Equal(t, 2, results[2].SortOrder)
	})
}

func TestUpdateAppOrder_AllAppsGetSortOrder(t *testing.T) {
	testutil.TestTransaction(t, func(db *gorm.DB) {
		vendor := testutil.NewTestVendor()
		require.NoError(t, db.Create(vendor).Error)
		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID})
		require.NoError(t, db.Create(org).Error)

		h := &Handler{db: db, logger: zap.NewNop()}

		// Mix of published and unpublished apps
		body, _ := json.Marshal(map[string]interface{}{
			"app_ids": []string{"app_pub", "app_unpub", "app_soon"},
			"app_statuses": map[string]string{
				"app_pub":   "published",
				"app_unpub": "unpublished",
				"app_soon":  "coming_soon",
			},
		})
		c, w := testutil.NewTestContextWithRequest(http.MethodPut, "/", body)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("org", org)
		h.UpdateAppOrder(c)
		assert.Equal(t, http.StatusOK, w.Code)

		var results []models.PublishedApp
		require.NoError(t, db.Where("org_id = ?", org.ID).Order("sort_order asc").Find(&results).Error)
		require.Len(t, results, 3)

		assert.Equal(t, "app_pub", results[0].AppID)
		assert.Equal(t, 0, results[0].SortOrder)
		assert.Equal(t, models.AppStatusPublished, results[0].Status)

		assert.Equal(t, "app_unpub", results[1].AppID)
		assert.Equal(t, 1, results[1].SortOrder)
		assert.Equal(t, models.AppStatusUnpublished, results[1].Status)

		assert.Equal(t, "app_soon", results[2].AppID)
		assert.Equal(t, 2, results[2].SortOrder)
		assert.Equal(t, models.AppStatusComingSoon, results[2].Status)
	})
}

func TestUpdateAppOrder_UnpublishAndRepublish(t *testing.T) {
	testutil.TestTransaction(t, func(db *gorm.DB) {
		vendor := testutil.NewTestVendor()
		require.NoError(t, db.Create(vendor).Error)
		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: vendor.ID})
		require.NoError(t, db.Create(org).Error)

		pa := models.PublishedApp{OrgID: org.ID, AppID: "app_aaa", SortOrder: 0, Status: "published"}
		require.NoError(t, db.Create(&pa).Error)

		h := &Handler{db: db, logger: zap.NewNop()}

		// Unpublish
		body1, _ := json.Marshal(map[string]interface{}{
			"app_ids":      []string{"app_aaa"},
			"app_statuses": map[string]string{"app_aaa": "unpublished"},
		})
		c1, w1 := testutil.NewTestContextWithRequest(http.MethodPut, "/", body1)
		c1.Request.Header.Set("Content-Type", "application/json")
		c1.Set("org", org)
		h.UpdateAppOrder(c1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Verify status changed to unpublished (not soft-deleted)
		var unpub models.PublishedApp
		require.NoError(t, db.Where("org_id = ? AND app_id = ?", org.ID, "app_aaa").First(&unpub).Error)
		assert.Equal(t, models.AppStatusUnpublished, unpub.Status)

		// Republish
		body2, _ := json.Marshal(map[string]interface{}{
			"app_ids":      []string{"app_aaa"},
			"app_statuses": map[string]string{"app_aaa": "coming_soon"},
		})
		c2, w2 := testutil.NewTestContextWithRequest(http.MethodPut, "/", body2)
		c2.Request.Header.Set("Content-Type", "application/json")
		c2.Set("org", org)
		h.UpdateAppOrder(c2)
		assert.Equal(t, http.StatusOK, w2.Code)

		// Verify restored with correct status
		var restored models.PublishedApp
		require.NoError(t, db.Where("org_id = ? AND app_id = ?", org.ID, "app_aaa").First(&restored).Error)
		assert.Equal(t, "coming_soon", restored.Status)
		assert.Equal(t, 0, restored.SortOrder)
	})
}
