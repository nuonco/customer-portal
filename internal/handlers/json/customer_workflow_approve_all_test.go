package jsonhandlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/testutil"
)

func TestCustomerInstallWizardApproveAll_MissingInstallContext(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		org := testutil.NewTestOrg(testutil.OrgOptions{Subdomain: "acme"})
		require.NoError(t, tx.Create(org).Error)

		h := NewCustomerInstallWizardHandler(tx, "http://localhost:8080", "localhost:8080", "http://nuon.local")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/portal-api/installs/inst_1/workflows/wf_1/approve-all?subdomain=acme", nil)
		c.Params = gin.Params{{Key: "install_id", Value: "inst_1"}, {Key: "workflow_id", Value: "wf_1"}}

		h.ApproveAllWorkflowSteps(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
		assert.Equal(t, "Install not found or access denied", payload["error"])
	})
}

func TestCustomerInstallWizardApproveAll_MissingWorkflowID(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		org := testutil.NewTestOrg(testutil.OrgOptions{Subdomain: "acme"})
		require.NoError(t, tx.Create(org).Error)

		install := testutil.NewTestInstall(testutil.InstallOptions{OrgID: org.ID})
		require.NoError(t, tx.Create(install).Error)

		h := NewCustomerInstallWizardHandler(tx, "http://localhost:8080", "localhost:8080", "http://nuon.local")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/portal-api/installs/"+install.ID+"/workflows//approve-all?subdomain=acme", nil)
		c.Params = gin.Params{{Key: "install_id", Value: install.ID}, {Key: "workflow_id", Value: ""}}
		c.Set("install", install)

		h.ApproveAllWorkflowSteps(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
		assert.Equal(t, "Workflow ID is required", payload["error"])
	})
}
