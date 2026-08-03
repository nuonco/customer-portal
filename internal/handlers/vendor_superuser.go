package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/partials"
)

// SuperuserPanelContent renders the superuser sliding panel
func (h *Handler) SuperuserPanelContent(c *gin.Context) {
	user := h.GetFreshUser(c)
	props := partials.SuperuserPanelProps{
		User:     user,
		BasePath: h.basePath,
	}
	h.RenderTempl(c, http.StatusOK, partials.SuperuserPanel(props))
}

// SuperuserSearchOrgs searches orgs by name, subdomain, or nuon org ID
func (h *Handler) SuperuserSearchOrgs(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		h.RenderTempl(c, http.StatusOK, partials.SuperuserSearchResults(nil))
		return
	}

	var orgs []models.NuonOrg
	like := "%" + q + "%"
	h.db.Where("name ILIKE ? OR subdomain ILIKE ? OR org_id ILIKE ?", like, like, like).
		Limit(20).
		Find(&orgs)

	h.RenderTempl(c, http.StatusOK, partials.SuperuserSearchResults(orgs))
}

// SuperuserOrgDetail loads org detail with membership check for the current user
func (h *Handler) SuperuserOrgDetail(c *gin.Context) {
	orgID := c.Param("org_id")
	user := middleware.GetCurrentUser(c)

	var org models.NuonOrg
	if err := h.db.First(&org, "id = ?", orgID).Error; err != nil {
		c.String(http.StatusNotFound, "Org not found")
		return
	}

	var count int64
	h.db.Model(&models.OrgMember{}).
		Where("user_id = ? AND org_id = ? AND status = ?", user.ID, orgID, models.MemberStatusActive).
		Count(&count)

	props := partials.SuperuserOrgDetailProps{
		Org:      &org,
		IsMember: count > 0,
		BasePath: h.basePath,
	}
	h.RenderTempl(c, http.StatusOK, partials.SuperuserOrgDetail(props))
}

// SuperuserJoinOrg creates an OrgMember for the current user
func (h *Handler) SuperuserJoinOrg(c *gin.Context) {
	orgID := c.Param("org_id")
	user := middleware.GetCurrentUser(c)

	var org models.NuonOrg
	if err := h.db.First(&org, "id = ?", orgID).Error; err != nil {
		c.String(http.StatusNotFound, "Org not found")
		return
	}

	member := models.OrgMember{
		UserID: user.ID,
		OrgID:  orgID,
		Status: models.MemberStatusActive,
	}
	if err := h.db.Where("user_id = ? AND org_id = ?", user.ID, orgID).
		Assign(models.OrgMember{Status: models.MemberStatusActive}).
		FirstOrCreate(&member).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to join org")
		return
	}

	props := partials.SuperuserOrgDetailProps{
		Org:      &org,
		IsMember: true,
		BasePath: h.basePath,
	}
	h.RenderTempl(c, http.StatusOK, partials.SuperuserOrgDetail(props))
}

// SuperuserLeaveOrg removes the OrgMember for the current user
func (h *Handler) SuperuserLeaveOrg(c *gin.Context) {
	orgID := c.Param("org_id")
	user := middleware.GetCurrentUser(c)

	var org models.NuonOrg
	if err := h.db.First(&org, "id = ?", orgID).Error; err != nil {
		c.String(http.StatusNotFound, "Org not found")
		return
	}

	h.db.Where("user_id = ? AND org_id = ?", user.ID, orgID).Delete(&models.OrgMember{})

	props := partials.SuperuserOrgDetailProps{
		Org:      &org,
		IsMember: false,
		BasePath: h.basePath,
	}
	h.RenderTempl(c, http.StatusOK, partials.SuperuserOrgDetail(props))
}
