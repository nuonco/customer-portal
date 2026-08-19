package handlers

import (
	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
)

// PublishApp publishes an app so customers can discover and install it without a link

func (h *Handler) orgHasPublishedApps(orgID string) bool {
	var count int64
	h.db.Model(&models.PublishedApp{}).Where("org_id = ? AND status IN ?", orgID, []string{models.AppStatusPublished, models.AppStatusComingSoon}).Count(&count)
	return count > 0
}

// AppsPage displays the vendor apps list
