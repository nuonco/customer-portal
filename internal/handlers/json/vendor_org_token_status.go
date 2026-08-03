package jsonhandlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

type orgTokenStatusResponse struct {
	IsValid      bool   `json:"is_valid"`
	ErrorTitle   string `json:"error_title,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// OrgTokenStatus validates the current org API token against the Nuon API.
func (h *VendorHandler) OrgTokenStatus(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organization context not found"})
		return
	}

	if org.APIToken == "" {
		c.JSON(http.StatusOK, orgTokenStatusResponse{
			IsValid:      false,
			ErrorTitle:   "No API token configured",
			ErrorMessage: "Please add an API token for this organization",
		})
		return
	}

	checkCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	client, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		c.JSON(http.StatusOK, orgTokenStatusResponse{
			IsValid:      false,
			ErrorTitle:   "Unable to reach Nuon API",
			ErrorMessage: "Unable to initialize Nuon client",
		})
		return
	}

	if err := client.ValidateOrgAccess(checkCtx); err != nil {
		apiErr := nuon.ParseAPIError(err)
		c.JSON(http.StatusOK, orgTokenStatusResponse{
			IsValid:      false,
			ErrorTitle:   apiErr.Title,
			ErrorMessage: apiErr.Description,
		})
		return
	}

	c.JSON(http.StatusOK, orgTokenStatusResponse{IsValid: true})
}
