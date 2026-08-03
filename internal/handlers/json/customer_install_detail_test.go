package jsonhandlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/testutil"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

func TestCustomerInstallDetail_ReturnsInstallAndReadmeDataWithoutNuonToken(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		customer := testutil.NewTestCustomer()
		require.NoError(t, tx.Create(customer).Error)

		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: customer.ID, Name: "Acme Cloud", Subdomain: "acme"})
		require.NoError(t, tx.Create(org).Error)

		account := testutil.NewTestCustomerAccount(testutil.CustomerAccountOptions{
			OrgID:           org.ID,
			Name:            "Platform Team",
			CreatedByUserID: customer.ID,
		})
		require.NoError(t, tx.Create(account).Error)

		require.NoError(t, tx.Create(testutil.NewTestCustomerAccountMember(testutil.CustomerAccountMemberOptions{
			AccountID: account.ID,
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
		install.CustomerAccountID = &account.ID
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

		h := NewCustomerInstallDetailHandler(tx, "http://localhost:8081")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "install_id", Value: install.ID}}
		c.Request = httptest.NewRequest(http.MethodGet, "/portal-api/installs/"+install.ID+"/detail?subdomain=acme", nil)
		c.Set("JWT_PAYLOAD", jwt.MapClaims{
			"user_id": customer.ID,
			"name":    customer.Name,
			"email":   customer.Email,
			"role":    string(customer.Role),
		})

		h.Detail(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response customerInstallDetailResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Equal(t, install.ID, response.Install.ID)
		assert.Equal(t, "Payments", response.App.DisplayName)
		assert.Equal(t, "data:image/png;base64,app-light", response.App.LogoLight)
		assert.Equal(t, "# Payments\nDeploy the payments stack.", response.Readme.Markdown)
		assert.Equal(t, "/bff/installs/"+install.ID, response.LegacyBasePath)
	})
}

func TestCustomerInstallDetail_RejectsUnknownInstall(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		customer := testutil.NewTestCustomer()
		require.NoError(t, tx.Create(customer).Error)

		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: customer.ID, Name: "Acme Cloud", Subdomain: "acme"})
		require.NoError(t, tx.Create(org).Error)

		h := NewCustomerInstallDetailHandler(tx, "http://localhost:8081")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "install_id", Value: "inst_missing"}}
		c.Request = httptest.NewRequest(http.MethodGet, "/portal-api/installs/inst_missing/detail?subdomain=acme", nil)
		c.Set("JWT_PAYLOAD", jwt.MapClaims{
			"user_id": customer.ID,
			"name":    customer.Name,
			"email":   customer.Email,
			"role":    string(customer.Role),
		})

		h.Detail(c)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestBuildComponentInfosFromDeployStatusMap_SortsAndBuildsFallbackItems(t *testing.T) {
	infos := buildComponentInfosFromDeployStatusMap(map[string]string{
		"comp-b": "error",
		"comp-a": "completed",
	})

	require.Len(t, infos, 2)
	assert.Equal(t, "comp-a", infos[0].Name)
	assert.Equal(t, "completed", infos[0].Status)
	assert.Equal(t, "comp-b", infos[1].Name)
	assert.Equal(t, "error", infos[1].Status)

	names := []string{infos[0].Name, infos[1].Name}
	sorted := append([]string{}, names...)
	sort.Strings(sorted)
	assert.Equal(t, sorted, names)
}

func TestBuildComponentInfosFromWorkflowGroups_IncludesComponentGroups(t *testing.T) {
	infos := buildComponentInfosFromWorkflowGroups(
		[]nuon.WorkflowStepGroup{
			{
				Name: "provision-install-stack",
				Labels: map[string]string{
					"name": "provision-install-stack",
				},
				Steps: []*nuon.WorkflowStep{{
					ExecutionType: "apply",
					Status:        &nuon.CompositeStatus{Status: "completed"},
				}},
			},
			{
				Name: "deploy-api",
				Labels: map[string]string{
					"domain":         "component",
					"component_name": "api",
				},
				Steps: []*nuon.WorkflowStep{{
					ExecutionType: "apply",
					Status:        &nuon.CompositeStatus{Status: "error"},
				}},
			},
			{
				Name: "deploy-worker",
				Labels: map[string]string{
					"domain":         "component",
					"component_name": "worker",
				},
				Steps: []*nuon.WorkflowStep{{
					ExecutionType: "apply",
					Status:        &nuon.CompositeStatus{Status: "completed"},
				}},
			},
		},
		map[string]string{
			"api":    "terraform",
			"worker": "docker_build",
		},
		map[string]string{
			"cmp_api":    "terraform",
			"cmp_worker": "docker_build",
		},
		map[string]string{
			"cmp_api":    "api",
			"cmp_worker": "worker",
		},
		map[string]string{
			"api":    "cmp_api",
			"worker": "cmp_worker",
		},
	)

	require.Len(t, infos, 2)
	assert.Equal(t, "api", infos[0].Name)
	assert.Equal(t, "cmp_api", infos[0].ID)
	assert.Equal(t, "error", infos[0].Status)
	assert.Equal(t, "terraform", infos[0].Type)
	assert.Equal(t, "worker", infos[1].Name)
	assert.Equal(t, "cmp_worker", infos[1].ID)
	assert.Equal(t, "completed", infos[1].Status)
	assert.Equal(t, "docker_build", infos[1].Type)
}

func TestBuildComponentInfosFromWorkflowGroups_UsesBackfilledComponentName(t *testing.T) {
	infos := buildComponentInfosFromWorkflowGroups(
		[]nuon.WorkflowStepGroup{
			{
				Name: "deploy-billing",
				Labels: map[string]string{
					"component_name": "billing",
				},
				Steps: []*nuon.WorkflowStep{{
					ExecutionType: "apply",
					Status:        &nuon.CompositeStatus{Status: "completed"},
				}},
			},
		},
		map[string]string{"billing": "helm"},
		map[string]string{"cmp_billing": "helm"},
		map[string]string{"cmp_billing": "billing"},
		map[string]string{"billing": "cmp_billing"},
	)

	require.Len(t, infos, 1)
	assert.Equal(t, "billing", infos[0].Name)
	assert.Equal(t, "cmp_billing", infos[0].ID)
	assert.Equal(t, "completed", infos[0].Status)
	assert.Equal(t, "helm", infos[0].Type)
}

func TestBuildComponentInfosFromWorkflowGroups_DoesNotUseWorkflowGroupIDAsName(t *testing.T) {
	infos := buildComponentInfosFromWorkflowGroups(
		[]nuon.WorkflowStepGroup{
			{
				ID:   "wsg5qtepmr75ksujbdobqd46iv",
				Name: "wsg5qtepmr75ksujbdobqd46iv",
				Steps: []*nuon.WorkflowStep{{
					ExecutionType: "apply",
					Status:        &nuon.CompositeStatus{Status: "completed"},
				}},
			},
		},
		map[string]string{},
		map[string]string{},
		map[string]string{},
		map[string]string{},
	)

	require.Len(t, infos, 1)
	assert.Equal(t, "wsg5qtepmr75ksujbdobqd46iv", infos[0].ID)
	assert.Equal(t, "Unknown component", infos[0].Name)
	assert.Equal(t, "completed", infos[0].Status)
}
