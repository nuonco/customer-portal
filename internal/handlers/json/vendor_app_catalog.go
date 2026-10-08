package jsonhandlers

import (
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/customer-portal/internal/middleware"
	"github.com/nuonco/customer-portal/internal/models"
	"github.com/nuonco/customer-portal/pkg/nuon"
)

type appCatalogItemResponse struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Platform        string `json:"platform"`
	LogoLightBase64 string `json:"logo_light_base64"`
	LogoDarkBase64  string `json:"logo_dark_base64"`
	Deleted         bool   `json:"deleted"`
	Status          string `json:"status"`
}

func (h *VendorHandler) AppCatalog(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Organization context not found"})
		return
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURLForOrg(org))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize Nuon client"})
		return
	}

	const appsPerPage = 10
	page := pageFromQuery(c)
	offset := (page - 1) * appsPerPage

	apps, hasMore, err := nuonClient.ListAppsPaginated(c.Request.Context(), offset, appsPerPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch apps"})
		return
	}

	var publishedApps []models.PublishedApp
	if err := h.db.Where("org_id = ?", org.ID).Order("sort_order ASC, created_at ASC").Find(&publishedApps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch published apps"})
		return
	}

	const maxInt = int(^uint(0) >> 1)

	statusMap := make(map[string]string, len(publishedApps))
	sortOrderMap := make(map[string]int, len(publishedApps))
	logoLightMap := make(map[string]string, len(publishedApps))
	logoDarkMap := make(map[string]string, len(publishedApps))

	for _, app := range publishedApps {
		status := app.Status
		if status == "" {
			status = models.AppStatusPublished
		}
		statusMap[app.AppID] = status
		sortOrderMap[app.AppID] = app.SortOrder
		if app.LogoLightBase64 != "" {
			logoLightMap[app.AppID] = app.LogoLightBase64
		}
		if app.LogoDarkBase64 != "" {
			logoDarkMap[app.AppID] = app.LogoDarkBase64
		}
	}

	catalogItems := make([]appCatalogItemResponse, 0, len(apps)+len(publishedApps))
	for _, app := range apps {
		platform := "aws"
		if app.RunnerConfig != nil {
			runnerType := string(app.RunnerConfig.AppRunnerType)
			switch runnerType {
			case "azure":
				platform = "azure"
			case "gcp":
				platform = "gcp"
			}
		}

		status := statusMap[app.ID]
		if status == "" {
			status = models.AppStatusUnpublished
		}

		catalogItems = append(catalogItems, appCatalogItemResponse{
			ID:              app.ID,
			Name:            app.Name,
			Platform:        platform,
			LogoLightBase64: logoLightMap[app.ID],
			LogoDarkBase64:  logoDarkMap[app.ID],
			Deleted:         false,
			Status:          status,
		})
	}

	if page == 1 && !hasMore {
		apiAppIDs := make(map[string]bool, len(apps))
		for _, app := range apps {
			apiAppIDs[app.ID] = true
		}

		for _, publishedApp := range publishedApps {
			if apiAppIDs[publishedApp.AppID] {
				continue
			}

			status := statusMap[publishedApp.AppID]
			if status == "" {
				status = models.AppStatusUnpublished
			}

			catalogItems = append(catalogItems, appCatalogItemResponse{
				ID:              publishedApp.AppID,
				Name:            "-",
				Platform:        "",
				LogoLightBase64: publishedApp.LogoLightBase64,
				LogoDarkBase64:  publishedApp.LogoDarkBase64,
				Deleted:         true,
				Status:          status,
			})
		}
	}

	sort.SliceStable(catalogItems, func(i, j int) bool {
		leftOrder, leftExists := sortOrderMap[catalogItems[i].ID]
		rightOrder, rightExists := sortOrderMap[catalogItems[j].ID]

		if !leftExists {
			leftOrder = maxInt
		}
		if !rightExists {
			rightOrder = maxInt
		}

		return leftOrder < rightOrder
	})

	c.JSON(http.StatusOK, gin.H{
		"apps": catalogItems,
		"pagination": gin.H{
			"current_page":  page,
			"has_previous":  page > 1,
			"has_next":      hasMore,
			"previous_page": page - 1,
			"next_page":     page + 1,
		},
	})
}
