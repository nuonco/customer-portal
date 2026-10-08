package jsonhandlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/internal/testutil"
	"github.com/nuonco/customer-portal/pkg/nuon"
)

type testStatusCoderError struct {
	status int
}

func (e testStatusCoderError) Error() string {
	return "status coder error"
}

func (e testStatusCoderError) IsCode(code int) bool {
	return code == e.status
}

func TestCustomerInstallWizardState_ReturnsConfigureData(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		customer := testutil.NewTestCustomer()
		require.NoError(t, tx.Create(customer).Error)

		org := testutil.NewTestOrg(testutil.OrgOptions{UserID: customer.ID, Name: "Acme Cloud", Subdomain: "acme"})
		require.NoError(t, tx.Create(org).Error)

		publishedApp := &models.PublishedApp{
			OrgID:            org.ID,
			AppID:            "app-payments",
			Status:           models.AppStatusPublished,
			LogoLightBase64:  "data:image/png;base64,app-light",
			LogoDarkBase64:   "data:image/png;base64,app-dark",
			OverviewMarkdown: "# Payments\nDeploy the payments stack.",
		}
		require.NoError(t, tx.Create(publishedApp).Error)

		h := NewCustomerInstallWizardHandler(tx, "http://localhost:8080", "localhost:8080", "http://nuon.local")

		w := httptest.NewRecorder()
		c, router := gin.CreateTestContext(w)
		router.GET("/portal-api/apps/:app_id/install-wizard", h.State)
		c.Request = httptest.NewRequest(http.MethodGet, "/portal-api/apps/app-payments/install-wizard?subdomain=acme", nil)
		c.Params = gin.Params{{Key: "app_id", Value: "app-payments"}}
		c.Set("JWT_PAYLOAD", jwt.MapClaims{
			"user_id": customer.ID,
			"name":    customer.Name,
			"email":   customer.Email,
			"role":    string(customer.Role),
		})

		h.State(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response customerWizardStateResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Equal(t, "app-payments", response.App.AppID)
		assert.Equal(t, "Payments", response.App.DisplayName)
		require.NotNil(t, response.Form)
		assert.Empty(t, response.WorkflowID)
		assert.Equal(t, "Deploy the payments stack.", response.App.Summary)
	})
}

func TestBuildWizardCreateInstallError_Conflict(t *testing.T) {
	status, message := buildWizardCreateInstallError(testStatusCoderError{status: 409}, "install-test")

	assert.Equal(t, http.StatusConflict, status)
	assert.Equal(t, "Install name \"install-test\" is already in use. Please choose a different name.", message)
}

func TestBuildWizardCreateInstallError_Fallback(t *testing.T) {
	status, message := buildWizardCreateInstallError(errors.New("boom"), "install-test")

	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "Unable to create this install right now. Please try again.", message)
}

func TestEnsureLocalWizardUser_CreatesMissingUser(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		h := NewCustomerInstallWizardHandler(tx, "http://localhost:8080", "localhost:8080", "http://nuon.local")

		user := &models.User{
			ID:    "iurmissingwizarduser0000001",
			Name:  "Missing User",
			Email: "missing-user@example.com",
			Role:  models.RoleCustomer,
		}

		resolved, err := h.ensureLocalWizardUser(user)
		require.NoError(t, err)
		require.NotNil(t, resolved)
		assert.Equal(t, user.ID, resolved.ID)

		var stored models.User
		require.NoError(t, tx.Where("id = ?", user.ID).First(&stored).Error)
		assert.Equal(t, user.Email, stored.Email)
		assert.Equal(t, user.Role, stored.Role)
	})
}

func TestEnsureLocalWizardUser_RestoresSoftDeletedUser(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		h := NewCustomerInstallWizardHandler(tx, "http://localhost:8080", "localhost:8080", "http://nuon.local")

		deleted := &models.User{
			ID:    "iursoftdeletedwizarduser0001",
			Name:  "Old Name",
			Email: "soft-deleted@example.com",
			Role:  models.RoleVendor,
		}
		require.NoError(t, tx.Create(deleted).Error)
		require.NoError(t, tx.Delete(deleted).Error)

		updatedClaimsUser := &models.User{
			ID:    deleted.ID,
			Name:  "Restored Name",
			Email: "soft-deleted@example.com",
			Role:  models.RoleCustomer,
		}

		resolved, err := h.ensureLocalWizardUser(updatedClaimsUser)
		require.NoError(t, err)
		require.NotNil(t, resolved)

		var restored models.User
		require.NoError(t, tx.Where("id = ?", deleted.ID).First(&restored).Error)
		assert.Equal(t, "Restored Name", restored.Name)
		assert.Equal(t, models.RoleCustomer, restored.Role)
		assert.False(t, restored.DeletedAt.Valid)

		// Ensure row truly exists in default scope and wasn't recreated with a new ID.
		assert.Equal(t, deleted.ID, restored.ID)
		assert.WithinDuration(t, time.Now(), restored.UpdatedAt, 2*time.Minute)
	})
}

func TestEnsureLocalWizardUser_ReusesExistingUserByEmailWhenClaimIDDiffers(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		h := NewCustomerInstallWizardHandler(tx, "http://localhost:8080", "localhost:8080", "http://nuon.local")

		existing := &models.User{
			ID:    "iurexistinguserbyemail000001",
			Name:  "Existing User",
			Email: "same-email@example.com",
			Role:  models.RoleCustomer,
		}
		require.NoError(t, tx.Create(existing).Error)

		claimsUser := &models.User{
			ID:    "not-a-local-short-id-from-jwt",
			Name:  "Updated Name",
			Email: existing.Email,
			Role:  models.RoleCustomer,
		}

		resolved, err := h.ensureLocalWizardUser(claimsUser)
		require.NoError(t, err)
		require.NotNil(t, resolved)
		assert.Equal(t, existing.ID, resolved.ID)

		var users []models.User
		require.NoError(t, tx.Where("email = ?", existing.Email).Find(&users).Error)
		require.Len(t, users, 1)
		assert.Equal(t, existing.ID, users[0].ID)
		assert.Equal(t, "Updated Name", users[0].Name)
	})
}

func TestIsNoOpComponentsStep(t *testing.T) {
	allGroups := []nuon.WorkflowStepGroup{
		{
			Labels: map[string]string{"name": "provision-install-stack"},
			Steps: []*nuon.WorkflowStep{
				{ExecutionType: "apply", Status: &nuon.CompositeStatus{Status: "completed"}},
			},
		},
		{
			Labels: map[string]string{"name": "provision-sandbox"},
			Steps: []*nuon.WorkflowStep{
				{ExecutionType: "apply", Status: &nuon.CompositeStatus{Status: "completed"}},
			},
		},
	}

	assert.True(t, isNoOpComponentsStep("components", []nuon.WorkflowStepGroup{}, allGroups))
	assert.False(t, isNoOpComponentsStep("sandbox", []nuon.WorkflowStepGroup{}, allGroups))
}
