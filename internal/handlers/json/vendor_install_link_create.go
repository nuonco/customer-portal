package jsonhandlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/pkg/nuon"
	"go.uber.org/zap"
)

func (h *VendorHandler) CreateInstallLink(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	org := middleware.GetCurrentOrg(c)

	var req struct {
		AppID   string            `json:"app_id" binding:"required"`
		AppName string            `json:"app_name" binding:"required"`
		Name    string            `json:"name" binding:"required"`
		Inputs  map[string]string `json:"inputs"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	normalizedName := models.NormalizeInstallName(req.Name)
	if err := models.ValidateInstallName(normalizedName); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	localAvailable, err := models.IsInstallNameAvailableLocally(h.db, org.ID, req.AppID, normalizedName, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check install name availability"})
		return
	}
	if !localAvailable {
		c.JSON(http.StatusConflict, gin.H{"error": "An install link with this name already exists"})
		return
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	nuonAvailable, err := nuonClient.IsInstallNameAvailable(c.Request.Context(), req.AppID, normalizedName)
	if err != nil {
		zap.L().Warn("failed to check Nuon API for install name availability", zap.Error(err))
	} else if !nuonAvailable {
		c.JSON(http.StatusConflict, gin.H{"error": "An install with this name already exists"})
		return
	}

	sha, err := generateInstallLinkSHA()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate link"})
		return
	}

	link := models.InstallLink{
		UserID:  user.ID,
		OrgID:   org.ID,
		AppID:   req.AppID,
		AppName: req.AppName,
		Name:    normalizedName,
		SHA:     sha,
		Used:    false,
	}

	if req.Inputs != nil && len(req.Inputs) > 0 {
		if err := link.SetVendorInputs(req.Inputs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store vendor inputs"})
			return
		}
	}

	if err := h.db.Create(&link).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create install link"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"link": link})
}

func generateInstallLinkSHA() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
