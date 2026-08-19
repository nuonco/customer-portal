package handlers

import (
	vendorpages "github.com/nuonco/mono/services/customer-dashboard/internal/views/vendorui/pages"
	nuonmodels "github.com/nuonco/nuon/sdks/nuon-go/models"
)

// appDisplayName returns the human-readable name for an app,
// preferring DisplayName over Name.
func appDisplayName(app *nuonmodels.AppApp) string {
	if app.DisplayName != "" {
		return app.DisplayName
	}
	return app.Name
}

// appInfoDisplayName returns the display name from an AppInfo,
// falling back to the ID if the name is empty.
func appInfoDisplayName(info vendorpages.AppInfo) string {
	if info.Name != "" {
		return info.Name
	}
	return info.ID
}
