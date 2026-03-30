package pages

import "github.com/nuonco/mono/services/customer-dashboard/internal/models"

// getLoginPreviewRightSideStyle returns the CSS style for the login preview right panel.
// Priority: custom image > custom gradient > primary color fallback.
func getLoginPreviewRightSideStyle(theme *models.AppTheme) string {
	if theme.GetLoginRightSideImage() != "" {
		return "background-image: url(" + theme.GetLoginRightSideImage() + "); background-size: cover; background-position: center; position: relative;"
	}
	if theme.GetLoginRightSideGradient() != "" {
		return "background: " + theme.GetLoginRightSideGradient() + "; position: relative;"
	}
	return "background: linear-gradient(135deg, var(--preview-primary), var(--preview-primary)); position: relative;"
}
