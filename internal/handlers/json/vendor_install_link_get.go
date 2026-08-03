package jsonhandlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/shortid"
)

type installLinkDetailResponse struct {
	Link                        models.InstallLink `json:"link"`
	InstallURL                  string             `json:"install_url"`
	CustomerDashboardInstallURL string             `json:"customer_dashboard_install_url"`
	NuonDashboardInstallURL     string             `json:"nuon_dashboard_install_url"`
}

func constructCustomerDashboardInstallURL(baseURL, baseDomain, subdomain, installID string) string {
	if subdomain != "" && baseDomain != "" {
		scheme := "https://"
		if len(baseURL) >= 7 && baseURL[:7] == "http://" {
			scheme = "http://"
		}
		return fmt.Sprintf("%s%s.%s/installs/%s", scheme, subdomain, baseDomain, installID)
	}
	return fmt.Sprintf("%s/installs/%s", baseURL, installID)
}

func (h *VendorHandler) InstallLinkDetail(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	linkID := c.Param("link_id")

	if !shortid.IsValid(linkID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid link ID"})
		return
	}

	var link models.InstallLink
	if err := h.db.Preload("Install").Preload("Install.User").Preload("NuonOrg").Where("id = ? AND org_id = ?", linkID, org.ID).First(&link).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "install link not found"})
		return
	}

	installURL := link.GetInstallURLWithSubdomain(h.customerBaseURL, h.subdomainBaseDomain)

	var customerDashboardInstallURL string
	var nuonDashboardInstallURL string
	if link.Install != nil && link.NuonOrg.ID != "" {
		customerDashboardInstallURL = constructCustomerDashboardInstallURL(
			h.customerBaseURL,
			h.subdomainBaseDomain,
			link.NuonOrg.Subdomain,
			link.Install.ID,
		)

		if link.NuonOrg.NuonOrgID != "" && link.Install.NuonInstallID != "" {
			nuonDashboardInstallURL = fmt.Sprintf("https://app.nuon.co/%s/installs/%s", link.NuonOrg.NuonOrgID, link.Install.NuonInstallID)
		}
	}

	c.JSON(http.StatusOK, installLinkDetailResponse{
		Link:                        link,
		InstallURL:                  installURL,
		CustomerDashboardInstallURL: customerDashboardInstallURL,
		NuonDashboardInstallURL:     nuonDashboardInstallURL,
	})
}
