package jsonhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/internal/models"
)

type meOrgSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
}

type meOrgsResponse struct {
	Orgs []meOrgSummary `json:"orgs"`
}

func (h *VendorHandler) MeOrgs(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var memberships []models.OrgMember
	h.db.Where("user_id = ? AND status = ?", user.ID, models.MemberStatusActive).
		Preload("Org", "deleted_at IS NULL").Find(&memberships)

	summaries := make([]meOrgSummary, 0, len(memberships))
	for _, membership := range memberships {
		if membership.Org.ID == "" {
			continue
		}
		summaries = append(summaries, meOrgSummary{
			ID:        membership.Org.ID,
			Name:      membership.Org.Name,
			Subdomain: membership.Org.Subdomain,
		})
	}

	c.JSON(http.StatusOK, meOrgsResponse{Orgs: summaries})
}
