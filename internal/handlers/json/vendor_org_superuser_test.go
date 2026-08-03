package jsonhandlers

import (
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

func TestSuperuserJoinOrg_ReactivatesSoftDeletedMembership(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		user, org, err := testutil.SeedMinimalData(tx)
		require.NoError(t, err)

		err = tx.Where("user_id = ? AND org_id = ?", user.ID, org.ID).Delete(&models.OrgMember{}).Error
		require.NoError(t, err)

		h := NewVendorHandler(tx, "https://api.nuon.co", "http://localhost:8080", "localhost:8080")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/admin/superuser-api/orgs/"+org.ID+"/join", nil)
		c.Params = gin.Params{{Key: "org_id", Value: org.ID}}
		c.Set("JWT_PAYLOAD", jwt.MapClaims{
			"user_id": user.ID,
			"name":    user.Name,
			"email":   user.Email,
			"role":    string(user.Role),
		})

		h.SuperuserJoinOrg(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var member models.OrgMember
		err = tx.Unscoped().Where("user_id = ? AND org_id = ?", user.ID, org.ID).First(&member).Error
		require.NoError(t, err)
		assert.Equal(t, models.MemberStatusActive, member.Status)
		assert.False(t, member.DeletedAt.Valid)
	})
}
