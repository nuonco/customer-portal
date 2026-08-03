package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

type meResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type meOrgSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
}

type meOrgsResponse struct {
	Orgs []meOrgSummary `json:"orgs"`
}

func (h *Handler) Me(c *gin.Context) {
	user := middleware.GetCurrentUser(c)

	var dbUser models.User
	if err := h.db.First(&dbUser, "id = ?", user.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user"})
		return
	}

	c.JSON(http.StatusOK, meResponse{
		ID:    dbUser.ID,
		Email: dbUser.Email,
		Name:  dbUser.Name,
	})
}

func (h *Handler) MeOrgs(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	orgs := h.GetUserOrgs(user.ID)

	summaries := make([]meOrgSummary, 0, len(orgs))
	for _, org := range orgs {
		summaries = append(summaries, meOrgSummary{
			ID:        org.ID,
			Name:      org.Name,
			Subdomain: org.Subdomain,
		})
	}

	c.JSON(http.StatusOK, meOrgsResponse{Orgs: summaries})
}
