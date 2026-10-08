package jsonhandlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/models"
	"gorm.io/gorm"
)

type VendorHandler struct {
	db                  *gorm.DB
	nuonAPIURL          string
	customerBaseURL     string
	subdomainBaseDomain string
}

func NewVendorHandler(db *gorm.DB, nuonAPIURL, customerBaseURL, subdomainBaseDomain string) *VendorHandler {
	return &VendorHandler{
		db:                  db,
		nuonAPIURL:          nuonAPIURL,
		customerBaseURL:     customerBaseURL,
		subdomainBaseDomain: subdomainBaseDomain,
	}
}

func (h *VendorHandler) nuonAPIURLForOrg(org *models.NuonOrg) string {
	if org != nil && org.APIURL != "" {
		return org.APIURL
	}
	return h.nuonAPIURL
}

func pageFromQuery(c *gin.Context) int {
	pageStr := c.DefaultQuery("page", "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		return 1
	}
	return page
}
