package background

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nuonco/mono/services/customer-dashboard/internal/models"
	"github.com/nuonco/mono/services/customer-dashboard/pkg/nuon"
)

// HealthCheckRunner monitors installs and triggers health checks when they become active
type HealthCheckRunner struct {
	db         *gorm.DB
	interval   time.Duration
	nuonAPIURL string
	logger     *zap.Logger
	stopCh     chan struct{}
}

// NewHealthCheckRunner creates a new health check runner
func NewHealthCheckRunner(db *gorm.DB, interval time.Duration, nuonAPIURL string, logger *zap.Logger) *HealthCheckRunner {
	return &HealthCheckRunner{
		db:         db,
		interval:   interval,
		nuonAPIURL: nuonAPIURL,
		logger:     logger,
		stopCh:     make(chan struct{}),
	}
}

// Start begins the background health check monitoring loop
func (r *HealthCheckRunner) Start() {
	r.logger.Info("health check runner started", zap.Duration("interval", r.interval))
	go r.run()
}

// Stop stops the background health check monitoring loop
func (r *HealthCheckRunner) Stop() {
	close(r.stopCh)
	r.logger.Info("health check runner stopped")
}

func (r *HealthCheckRunner) run() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Run immediately on start
	r.checkInProgressInstalls()

	for {
		select {
		case <-ticker.C:
			r.checkInProgressInstalls()
		case <-r.stopCh:
			return
		}
	}
}

// checkInProgressInstalls finds all pending and provisioning installs and syncs their status with Nuon API
func (r *HealthCheckRunner) checkInProgressInstalls() {
	ctx := context.Background()

	// Find all installs in pending or provisioning status
	// We check both because the Nuon API is the source of truth for status transitions
	var installs []models.Install
	err := r.db.Preload("InstallLink").Preload("InstallLink.NuonOrg").
		Where("status IN ?", []models.InstallStatus{models.StatusPending, models.StatusProvisioning}).
		Find(&installs).Error

	if err != nil {
		r.logger.Error("error fetching in-progress installs", zap.Error(err))
		return
	}

	if len(installs) == 0 {
		return
	}

	r.logger.Debug("checking in-progress installs", zap.Int("count", len(installs)))

	for _, install := range installs {
		r.checkAndUpdateInstall(ctx, &install)
	}
}

// checkAndUpdateInstall checks a single install's status and triggers health checks if active
func (r *HealthCheckRunner) checkAndUpdateInstall(ctx context.Context, install *models.Install) {
	// Skip installs without an install link (published-app installs do not use health check configs)
	if install.InstallLinkID == nil {
		return
	}

	// Get install link
	var link models.InstallLink
	if err := r.db.First(&link, "id = ?", *install.InstallLinkID).Error; err != nil {
		r.logger.Error("failed to load install link",
			zap.String("install_link_id", *install.InstallLinkID),
			zap.Error(err),
		)
		return
	}
	install.InstallLink = link

	// Get org info directly by ID
	var org models.NuonOrg
	if err := r.db.First(&org, "id = ?", link.OrgID).Error; err != nil {
		r.logger.Error("failed to load org for install",
			zap.String("org_id", link.OrgID),
			zap.String("install_id", install.ID),
			zap.Error(err),
		)
		return
	}
	install.InstallLink.NuonOrg = org

	if org.APIToken == "" {
		r.logger.Warn("install has no org API token",
			zap.String("install_id", install.ID),
			zap.String("install_link_id", *install.InstallLinkID),
			zap.String("org_id", link.OrgID),
		)
		return
	}

	// Create Nuon client using global API URL
	client, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, r.nuonAPIURL)
	if err != nil {
		r.logger.Error("failed to create Nuon client",
			zap.String("install_id", install.ID),
			zap.Error(err),
		)
		return
	}

	// Get install status from Nuon API
	nuonInstall, err := client.GetInstall(ctx, install.NuonInstallID)
	if err != nil {
		r.logger.Error("failed to get Nuon install",
			zap.String("nuon_install_id", install.NuonInstallID),
			zap.Error(err),
		)
		return
	}

	// Map Nuon status to local status and handle transitions
	switch nuonInstall.Status {
	case "active", "deprecated":
		// "deprecated" means the install is running but on an old app version - treat as active
		if install.Status != models.StatusActive {
			r.logger.Info("install is now active, triggering health checks",
				zap.String("install_id", install.ID),
				zap.String("nuon_status", nuonInstall.Status),
			)
			if err := r.db.Model(install).Update("status", models.StatusActive).Error; err != nil {
				r.logger.Error("failed to update install status",
					zap.String("install_id", install.ID),
					zap.Error(err),
				)
				return
			}
			// Trigger health checks when transitioning to active
			r.triggerHealthChecks(ctx, client, install)
		}

	case "provisioning", "queued":
		if install.Status == models.StatusPending {
			r.logger.Info("install is now provisioning", zap.String("install_id", install.ID))
			if err := r.db.Model(install).Update("status", models.StatusProvisioning).Error; err != nil {
				r.logger.Error("failed to update install status",
					zap.String("install_id", install.ID),
					zap.Error(err),
				)
			}
		} else {
			r.logger.Debug("install still in progress",
				zap.String("install_id", install.ID),
				zap.String("status", nuonInstall.Status),
			)
		}

	case "error", "failed":
		r.logger.Info("install has failed, updating status", zap.String("install_id", install.ID))
		if err := r.db.Model(install).Update("status", models.StatusFailed).Error; err != nil {
			r.logger.Error("failed to update install status",
				zap.String("install_id", install.ID),
				zap.Error(err),
			)
		}

	default:
		r.logger.Debug("install status unchanged",
			zap.String("install_id", install.ID),
			zap.String("status", nuonInstall.Status),
		)
	}
}

// triggerHealthChecks triggers all configured health check actions for an install
func (r *HealthCheckRunner) triggerHealthChecks(ctx context.Context, client *nuon.Client, install *models.Install) {
	// Use the new GetHealthCheckIDPairs to get parsed config/workflow IDs
	healthCheckPairs := install.InstallLink.GetHealthCheckIDPairs()

	if len(healthCheckPairs) == 0 {
		r.logger.Warn("no health checks configured for install", zap.String("install_id", install.ID))
		return
	}

	r.logger.Debug("triggering health checks",
		zap.String("install_id", install.ID),
		zap.Int("count", len(healthCheckPairs)),
	)

	for _, pair := range healthCheckPairs {
		// RunInstallAction expects the ActionWorkflowConfigID (not WorkflowID)
		err := client.RunInstallAction(ctx, install.NuonInstallID, pair.ConfigID)
		if err != nil {
			r.logger.Error("failed to trigger health check action",
				zap.String("config_id", pair.ConfigID),
				zap.String("install_id", install.ID),
				zap.Error(err),
			)
		} else {
			r.logger.Debug("triggered health check action",
				zap.String("config_id", pair.ConfigID),
				zap.String("install_id", install.ID),
			)
		}
	}
}

// CheckInstallHealthStatus retrieves the current health check status for an install
// Returns a map of action ID to status info
func CheckInstallHealthStatus(ctx context.Context, client *nuon.Client, install *models.Install) ([]HealthCheckStatus, string, error) {
	// Use the new GetHealthCheckIDPairs to get parsed config/workflow IDs
	healthCheckPairs := install.InstallLink.GetHealthCheckIDPairs()

	if len(healthCheckPairs) == 0 {
		return nil, "", nil
	}

	var statuses []HealthCheckStatus
	allPassed := true
	anyRunning := false
	anyPending := false

	for _, pair := range healthCheckPairs {
		status := HealthCheckStatus{
			ActionID: pair.WorkflowID, // Use WorkflowID for display
			Status:   "Pending",
		}

		zap.L().Debug("checking action workflow",
			zap.String("workflow_id", pair.WorkflowID),
			zap.String("config_id", pair.ConfigID),
			zap.String("install_id", install.NuonInstallID),
		)

		// Get action info and recent runs using WorkflowID (not ConfigID)
		// GetInstallActionRuns expects the ActionWorkflowID
		actionRuns, err := client.GetInstallActionRuns(ctx, install.NuonInstallID, pair.WorkflowID)
		if err != nil {
			zap.L().Error("failed to get action runs",
				zap.String("workflow_id", pair.WorkflowID),
				zap.Error(err),
			)
			status.StatusMessage = fmt.Sprintf("Error: %v", err)
			anyPending = true
		} else if actionRuns != nil && actionRuns.ActionWorkflow != nil {
			zap.L().Debug("got action runs",
				zap.Int("run_count", len(actionRuns.Runs)),
				zap.String("action_name", actionRuns.ActionWorkflow.Name),
			)
			status.ActionName = actionRuns.ActionWorkflow.Name
			status.ActionID = actionRuns.ActionWorkflow.ID

			// Check most recent run status
			if len(actionRuns.Runs) > 0 {
				recentRun := actionRuns.Runs[0]
				switch recentRun.Status {
				case "completed", "succeeded", "active", "finished":
					status.Status = "Passing"
					status.StatusClass = "bg-green-100 text-green-800"
				case "running", "in-progress", "queued":
					status.Status = "Pending"
					status.StatusClass = "bg-blue-100 text-blue-800"
					anyRunning = true
					allPassed = false
				case "failed", "error", "cancelled":
					status.Status = "Failing"
					status.StatusClass = "bg-red-100 text-red-800"
					allPassed = false
				default:
					status.Status = "Pending"
					status.StatusClass = "bg-gray-100 text-gray-800"
					anyPending = true
					allPassed = false
				}
				if recentRun.CreatedAt != "" {
					// Parse time string - ignore errors for now
					if t, err := time.Parse(time.RFC3339, recentRun.CreatedAt); err == nil {
						status.LastRunAt = t
					}
				}
			} else {
				// No runs yet
				anyPending = true
				allPassed = false
				status.StatusClass = "bg-gray-100 text-gray-800"
			}
		} else {
			anyPending = true
			allPassed = false
			status.StatusClass = "bg-gray-100 text-gray-800"
		}

		statuses = append(statuses, status)
	}

	// Determine overall status
	var overallStatus string
	if allPassed && len(statuses) > 0 {
		overallStatus = "passing"
	} else if anyRunning {
		overallStatus = "pending"
	} else if anyPending {
		overallStatus = "pending"
	} else {
		overallStatus = "failing"
	}

	return statuses, overallStatus, nil
}

// HealthCheckStatus represents the status of a single health check action
type HealthCheckStatus struct {
	ActionID      string    `json:"action_id"`
	ActionName    string    `json:"action_name"`
	Status        string    `json:"status"` // pending, running, passed, failed
	StatusClass   string    `json:"status_class"`
	StatusMessage string    `json:"status_message,omitempty"`
	LastRunAt     time.Time `json:"last_run_at,omitempty"`
}
