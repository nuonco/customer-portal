package jsonhandlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (h *VendorHandler) CreateOrg(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var req struct {
		Name     string `json:"name"`
		OrgID    string `json:"org_id" binding:"required"`
		APIToken string `json:"api_token" binding:"required"`
		APIURL   string `json:"api_url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	apiURL := h.nuonAPIURL
	if req.APIURL != "" {
		apiURL = req.APIURL
	}
	nuonClient, err := nuon.NewClientWithURL(req.APIToken, req.OrgID, apiURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	if err := nuonClient.ValidateOrgAccess(c.Request.Context()); err != nil {
		// Log the wrapped cause: "invalid API token" is indistinguishable from
		// "nothing listening on api_url" without it.
		zap.L().Warn("org connect validation failed",
			zap.String("api_url", apiURL),
			zap.String("org_id", req.OrgID),
			zap.Error(err))

		status, msg := nuon.DescribeConnectError(err, apiURL)
		c.JSON(status, gin.H{"error": msg})
		return
	}

	nuonOrg, err := nuonClient.GetOrg(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch organization details from Nuon API"})
		return
	}

	orgName := nuonOrg.Name
	if orgName == "" {
		orgName = req.OrgID
	}

	var existingOrg models.NuonOrg
	if err := h.db.Where("org_id = ? AND deleted_at IS NULL", req.OrgID).First(&existingOrg).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "This Nuon organization is already connected"})
		return
	}

	displayName := req.Name
	if displayName == "" {
		displayName = orgName
	}

	subdomain, err := models.GenerateUniqueSubdomain(h.db, orgName, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate subdomain"})
		return
	}

	var org models.NuonOrg
	err = h.db.Transaction(func(tx *gorm.DB) error {
		org = models.NuonOrg{
			UserID:    user.ID,
			NuonOrgID: req.OrgID,
			APIToken:  req.APIToken,
			APIURL:    req.APIURL,
			Name:      displayName,
			Subdomain: subdomain,
		}
		if err := tx.Create(&org).Error; err != nil {
			return fmt.Errorf("failed to create organization: %w", err)
		}

		now := time.Now()
		member := models.OrgMember{OrgID: org.ID, UserID: user.ID, Status: models.MemberStatusActive, JoinedAt: &now}
		if err := tx.Create(&member).Error; err != nil {
			return fmt.Errorf("failed to add user to organization: %w", err)
		}

		theme := models.AppTheme{OrgID: org.ID, PrimaryColor: models.DefaultPrimaryColor, BorderRadius: models.DefaultBorderRadius}
		if err := tx.Create(&theme).Error; err != nil {
			return fmt.Errorf("failed to create theme: %w", err)
		}

		authConfig := models.CustomerAuthConfig{OrgID: org.ID, Enabled: false, Scopes: models.DefaultScopes}
		if err := tx.Create(&authConfig).Error; err != nil {
			return fmt.Errorf("failed to create auth config: %w", err)
		}

		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create organization: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          org.ID,
		"name":        org.Name,
		"redirect_to": fmt.Sprintf("/admin/orgs/%s/install-links", org.ID),
	})
}
