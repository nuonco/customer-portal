package jsonhandlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/testutil"
)

func TestVendorUpdateProfile_Success(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		user, _, err := testutil.SeedMinimalData(tx)
		require.NoError(t, err)

		h := NewVendorHandler(tx, "https://api.nuon.co", "http://localhost:8080", "localhost:8080")

		body, err := json.Marshal(map[string]string{"name": "  Ada Lovelace  "})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPut, "/admin/profile", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("JWT_PAYLOAD", jwt.MapClaims{
			"user_id": user.ID,
			"name":    user.Name,
			"email":   user.Email,
			"role":    string(user.Role),
		})

		h.UpdateProfile(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var updatedName string
		err = tx.Raw("SELECT name FROM users WHERE id = ?", user.ID).Scan(&updatedName).Error
		require.NoError(t, err)
		assert.Equal(t, "Ada Lovelace", updatedName)
	})
}

func TestVendorUpdateProfile_Validation(t *testing.T) {
	testutil.RequireTestDB(t)

	testutil.TestTransaction(t, func(tx *gorm.DB) {
		user, _, err := testutil.SeedMinimalData(tx)
		require.NoError(t, err)

		h := NewVendorHandler(tx, "https://api.nuon.co", "http://localhost:8080", "localhost:8080")

		tests := []struct {
			name           string
			requestName    string
			expectedStatus int
			expectedError  string
		}{
			{
				name:           "rejects empty display name",
				requestName:    "   ",
				expectedStatus: http.StatusBadRequest,
				expectedError:  "Display name is required",
			},
			{
				name:           "rejects name longer than 255 chars",
				requestName:    strings.Repeat("a", 256),
				expectedStatus: http.StatusBadRequest,
				expectedError:  "Display name must be 255 characters or fewer",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				body, err := json.Marshal(map[string]string{"name": tc.requestName})
				require.NoError(t, err)

				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request, _ = http.NewRequest(http.MethodPut, "/admin/profile", bytes.NewReader(body))
				c.Request.Header.Set("Content-Type", "application/json")
				c.Set("JWT_PAYLOAD", jwt.MapClaims{
					"user_id": user.ID,
					"name":    user.Name,
					"email":   user.Email,
					"role":    string(user.Role),
				})

				h.UpdateProfile(c)

				assert.Equal(t, tc.expectedStatus, w.Code)
				assert.Contains(t, w.Body.String(), tc.expectedError)
			})
		}
	})
}
