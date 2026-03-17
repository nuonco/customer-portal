package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"go.uber.org/zap"
)

func (h *Handler) CreateInstallLink(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	org := middleware.GetCurrentOrg(c)

	var req struct {
		AppID   string            `json:"app_id" binding:"required"`
		AppName string            `json:"app_name" binding:"required"`
		Name    string            `json:"name" binding:"required"` // Required install name
		Inputs  map[string]string `json:"inputs"`                  // Vendor-facing inputs (stored for later)
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Normalize and validate install name
	normalizedName := models.NormalizeInstallName(req.Name)
	if err := models.ValidateInstallName(normalizedName); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check local uniqueness (unused install links with same name for this app)
	localAvailable, err := models.IsInstallNameAvailableLocally(h.db, org.ID, req.AppID, normalizedName, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check install name availability"})
		return
	}
	if !localAvailable {
		c.JSON(http.StatusConflict, gin.H{"error": "An install link with this name already exists"})
		return
	}

	// Check Nuon API uniqueness (existing installs with same name)
	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	nuonAvailable, err := nuonClient.IsInstallNameAvailable(c.Request.Context(), req.AppID, normalizedName)
	if err != nil {
		// Log error but don't fail - Nuon API check is best-effort
		h.logger.Warn("failed to check Nuon API for install name availability", zap.Error(err))
	} else if !nuonAvailable {
		c.JSON(http.StatusConflict, gin.H{"error": "An install with this name already exists"})
		return
	}

	// Generate unique SHA for the link
	sha, err := generateSHA()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate link"})
		return
	}

	// Create install link with vendor inputs stored
	link := models.InstallLink{
		UserID:  user.ID,
		OrgID:   org.ID,
		AppID:   req.AppID,
		AppName: req.AppName,
		Name:    normalizedName,
		SHA:     sha,
		Used:    false,
	}

	// Store vendor inputs for later use when customer accepts
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

	// Note: Install is NOT created here - it will be created when customer accepts the link
	// This allows customer to provide required customer inputs and choose region/location

	c.JSON(http.StatusCreated, gin.H{
		"link": link,
	})
}

// InstallLinkDetail shows details about a specific install link using Templ

func generateSHA() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateInstallLink handles creating a new install link
// Vendor provides vendor-facing inputs which are stored for later use when customer accepts
// The Install is created when customer accepts the link (not here)
