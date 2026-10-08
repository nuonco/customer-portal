package jsonhandlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/internal/models"
	"gorm.io/gorm/clause"
)

type superuserOrgSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
	NuonOrgID string `json:"nuon_org_id"`
	IsMember  bool   `json:"is_member,omitempty"`
}

type superuserSearchResponse struct {
	Orgs []superuserOrgSummary `json:"orgs"`
}

type superuserOrgResponse struct {
	Org superuserOrgSummary `json:"org"`
}

func toSuperuserOrg(org models.NuonOrg, isMember bool) superuserOrgSummary {
	return superuserOrgSummary{
		ID:        org.ID,
		Name:      org.Name,
		Subdomain: org.Subdomain,
		NuonOrgID: org.NuonOrgID,
		IsMember:  isMember,
	}
}

// SuperuserSearchOrgs searches orgs by name, subdomain, or nuon org ID.
func (h *VendorHandler) SuperuserSearchOrgs(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusOK, superuserSearchResponse{Orgs: []superuserOrgSummary{}})
		return
	}

	var orgs []models.NuonOrg
	like := "%" + q + "%"
	h.db.Where("name ILIKE ? OR subdomain ILIKE ? OR org_id ILIKE ?", like, like, like).
		Limit(20).
		Find(&orgs)

	results := make([]superuserOrgSummary, 0, len(orgs))
	for _, org := range orgs {
		results = append(results, toSuperuserOrg(org, false))
	}
	c.JSON(http.StatusOK, superuserSearchResponse{Orgs: results})
}

// SuperuserOrgDetail returns org details with membership status for the current user.
func (h *VendorHandler) SuperuserOrgDetail(c *gin.Context) {
	orgID := c.Param("org_id")
	user := middleware.GetCurrentUser(c)

	var org models.NuonOrg
	if err := h.db.First(&org, "id = ?", orgID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "org not found"})
		return
	}

	var count int64
	h.db.Model(&models.OrgMember{}).
		Where("user_id = ? AND org_id = ? AND status = ?", user.ID, orgID, models.MemberStatusActive).
		Count(&count)

	c.JSON(http.StatusOK, superuserOrgResponse{Org: toSuperuserOrg(org, count > 0)})
}

// SuperuserJoinOrg creates an active OrgMember for the current user.
func (h *VendorHandler) SuperuserJoinOrg(c *gin.Context) {
	orgID := c.Param("org_id")
	user := middleware.GetCurrentUser(c)

	var org models.NuonOrg
	if err := h.db.First(&org, "id = ?", orgID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "org not found"})
		return
	}

	member := models.OrgMember{
		UserID: user.ID,
		OrgID:  orgID,
		Status: models.MemberStatusActive,
	}
	if err := h.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "org_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"status":     models.MemberStatusActive,
			"deleted_at": nil,
		}),
	}).Create(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to join org"})
		return
	}

	c.JSON(http.StatusOK, superuserOrgResponse{Org: toSuperuserOrg(org, true)})
}

// SuperuserLeaveOrg removes the OrgMember for the current user.
func (h *VendorHandler) SuperuserLeaveOrg(c *gin.Context) {
	orgID := c.Param("org_id")
	user := middleware.GetCurrentUser(c)

	var org models.NuonOrg
	if err := h.db.First(&org, "id = ?", orgID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "org not found"})
		return
	}

	h.db.Where("user_id = ? AND org_id = ?", user.ID, orgID).Delete(&models.OrgMember{})

	c.JSON(http.StatusOK, superuserOrgResponse{Org: toSuperuserOrg(org, false)})
}
