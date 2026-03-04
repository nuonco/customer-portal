package handlers

import (
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nuonco/mono/services/customer-dashboard/internal/middleware"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
	"go.uber.org/zap"
)

func (h *Handler) SearchNuonInstalls(c *gin.Context) {
	org := middleware.GetCurrentOrg(c)
	if org == nil {
		c.String(http.StatusBadRequest, "Organization context not found")
		return
	}

	q := strings.TrimSpace(c.Query("q"))
	if len(q) < 3 {
		c.String(http.StatusOK, "")
		return
	}

	nuonClient, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, h.nuonAPIURL)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to initialize Nuon client")
		return
	}

	apps, err := nuonClient.ListApps(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to fetch apps")
		return
	}

	type searchResult struct {
		InstallID   string
		InstallName string
		AppID       string
		AppName     string
		Status      string
		Region      string
	}

	var results []searchResult
	for _, app := range apps {
		if len(results) >= 20 {
			break
		}
		installs, err := nuonClient.ListAppInstalls(c.Request.Context(), app.ID, q)
		if err != nil {
			zap.L().Warn("failed to search installs for app",
				zap.String("app_id", app.ID),
				zap.Error(err),
			)
			continue
		}
		for _, install := range installs {
			if len(results) >= 20 {
				break
			}
			installName := install.Name
			status := install.Status
			region := ""
			if install.AwsAccount != nil && install.AwsAccount.Region != "" {
				region = install.AwsAccount.Region
			} else if install.AzureAccount != nil && install.AzureAccount.Location != "" {
				region = install.AzureAccount.Location
			}
			results = append(results, searchResult{
				InstallID:   install.ID,
				InstallName: installName,
				AppID:       app.ID,
				AppName:     app.Name,
				Status:      status,
				Region:      region,
			})
		}
	}

	// Build HTML fragment
	if len(results) == 0 {
		c.Data(http.StatusOK, "text/html", []byte(`<div class="px-4 py-3 text-sm text-cool-grey-500 dark:text-cool-grey-400">No installs found.</div>`))
		return
	}

	htmlOut := ""
	for _, r := range results {
		displayText := r.InstallName + "  •  " + r.AppName
		if r.Status != "" {
			displayText += "  •  " + r.Status
		}
		htmlOut += fmt.Sprintf(`<div class="px-4 py-3 hover:bg-cool-grey-50 dark:hover:bg-dark-grey-800">
			<div class="flex items-center justify-between gap-3">
				<div>
					<div class="text-sm font-medium text-cool-grey-900 dark:text-white">%s</div>
					<div class="text-xs text-cool-grey-500 dark:text-cool-grey-400">%s &middot; %s</div>
				</div>
				<button
					type="button"
					data-install-id="%s"
					data-app-id="%s"
					data-display="%s"
					onclick="selectImportInstall(this.dataset.installId, this.dataset.appId, this.dataset.display)"
					class="shrink-0 text-xs font-medium text-primary-600 dark:text-primary-400 hover:underline"
				>
					Select &rarr;
				</button>
			</div>
		</div>`,
			r.InstallName, r.AppName, r.Region,
			html.EscapeString(r.InstallID), html.EscapeString(r.AppID), html.EscapeString(displayText),
		)
	}

	c.Data(http.StatusOK, "text/html", []byte(htmlOut))
}

// ImportInstall imports an existing Nuon install into the portal and assigns it to a customer.
