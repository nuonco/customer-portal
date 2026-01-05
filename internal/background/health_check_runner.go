package background

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/powertoolsdev/mono/exp/installer/app/internal/models"
	"github.com/powertoolsdev/mono/exp/installer/app/pkg/nuon"
)

// HealthCheckRunner monitors installs and triggers health checks when they become active
type HealthCheckRunner struct {
	db         *gorm.DB
	interval   time.Duration
	nuonAPIURL string
	stopCh     chan struct{}
}

// NewHealthCheckRunner creates a new health check runner
func NewHealthCheckRunner(db *gorm.DB, interval time.Duration, nuonAPIURL string) *HealthCheckRunner {
	return &HealthCheckRunner{
		db:         db,
		interval:   interval,
		nuonAPIURL: nuonAPIURL,
		stopCh:     make(chan struct{}),
	}
}

// Start begins the background health check monitoring loop
func (r *HealthCheckRunner) Start() {
	log.Printf("Health check runner started, polling every %v", r.interval)
	go r.run()
}

// Stop stops the background health check monitoring loop
func (r *HealthCheckRunner) Stop() {
	close(r.stopCh)
	log.Println("Health check runner stopped")
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
		log.Printf("Health check runner: error fetching in-progress installs: %v", err)
		return
	}

	if len(installs) == 0 {
		return
	}

	log.Printf("Health check runner: checking %d in-progress installs", len(installs))

	for _, install := range installs {
		r.checkAndUpdateInstall(ctx, &install)
	}
}

// checkAndUpdateInstall checks a single install's status and triggers health checks if active
func (r *HealthCheckRunner) checkAndUpdateInstall(ctx context.Context, install *models.Install) {
	// Get install link
	var link models.InstallLink
	if err := r.db.First(&link, "id = ?", install.InstallLinkID).Error; err != nil {
		log.Printf("Health check runner: failed to load install link %s: %v", install.InstallLinkID, err)
		return
	}
	install.InstallLink = link

	// Get org info directly by ID
	var org models.NuonOrg
	if err := r.db.First(&org, "id = ?", link.OrgID).Error; err != nil {
		log.Printf("Health check runner: failed to load org %s for install %s: %v", link.OrgID, install.ID, err)
		return
	}
	install.InstallLink.NuonOrg = org

	if org.APIToken == "" {
		log.Printf("Health check runner: install %s has no org API token (link=%s, orgID=%s)",
			install.ID, install.InstallLinkID, link.OrgID)
		return
	}

	// Create Nuon client using global API URL
	client, err := nuon.NewClientWithURL(org.APIToken, org.NuonOrgID, r.nuonAPIURL)
	if err != nil {
		log.Printf("Health check runner: failed to create Nuon client for install %s: %v", install.ID, err)
		return
	}

	// Get install status from Nuon API
	nuonInstall, err := client.GetInstall(ctx, install.NuonInstallID)
	if err != nil {
		log.Printf("Health check runner: failed to get Nuon install %s: %v", install.NuonInstallID, err)
		return
	}

	// Map Nuon status to local status and handle transitions
	switch nuonInstall.Status {
	case "active", "deprecated":
		// "deprecated" means the install is running but on an old app version - treat as active
		if install.Status != models.StatusActive {
			log.Printf("Health check runner: install %s is now active (nuon status=%s), updating status and triggering health checks", install.ID, nuonInstall.Status)
			if err := r.db.Model(install).Update("status", models.StatusActive).Error; err != nil {
				log.Printf("Health check runner: failed to update install %s status: %v", install.ID, err)
				return
			}
			// Trigger health checks when transitioning to active
			r.triggerHealthChecks(ctx, client, install)
		}

	case "provisioning", "queued":
		if install.Status == models.StatusPending {
			log.Printf("Health check runner: install %s is now provisioning, updating status", install.ID)
			if err := r.db.Model(install).Update("status", models.StatusProvisioning).Error; err != nil {
				log.Printf("Health check runner: failed to update install %s status: %v", install.ID, err)
			}
		} else {
			log.Printf("Health check runner: install %s still %s", install.ID, nuonInstall.Status)
		}

	case "error", "failed":
		log.Printf("Health check runner: install %s has failed, updating status", install.ID)
		if err := r.db.Model(install).Update("status", models.StatusFailed).Error; err != nil {
			log.Printf("Health check runner: failed to update install %s status: %v", install.ID, err)
		}

	default:
		log.Printf("Health check runner: install %s has status %s (no transition needed)", install.ID, nuonInstall.Status)
	}
}

// triggerHealthChecks triggers all configured health check actions for an install
func (r *HealthCheckRunner) triggerHealthChecks(ctx context.Context, client *nuon.Client, install *models.Install) {
	// Use the new GetHealthCheckIDPairs to get parsed config/workflow IDs
	healthCheckPairs := install.InstallLink.GetHealthCheckIDPairs()

	if len(healthCheckPairs) == 0 {
		log.Printf("Health check runner: no health checks configured for install %s", install.ID)
		return
	}

	log.Printf("Health check runner: triggering %d health checks for install %s", len(healthCheckPairs), install.ID)

	for _, pair := range healthCheckPairs {
		// RunInstallAction expects the ActionWorkflowConfigID (not WorkflowID)
		err := client.RunInstallAction(ctx, install.NuonInstallID, pair.ConfigID)
		if err != nil {
			log.Printf("Health check runner: failed to trigger action (config=%s) on install %s: %v", pair.ConfigID, install.ID, err)
		} else {
			log.Printf("Health check runner: triggered action (config=%s) on install %s", pair.ConfigID, install.ID)
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

		log.Printf("CheckInstallHealthStatus: Checking action workflow %s (config=%s) for install %s", pair.WorkflowID, pair.ConfigID, install.NuonInstallID)

		// Get action info and recent runs using WorkflowID (not ConfigID)
		// GetInstallActionRuns expects the ActionWorkflowID
		actionRuns, err := client.GetInstallActionRuns(ctx, install.NuonInstallID, pair.WorkflowID)
		if err != nil {
			log.Printf("Failed to get action runs for %s: %v", pair.WorkflowID, err)
			status.StatusMessage = fmt.Sprintf("Error: %v", err)
			anyPending = true
		} else if actionRuns != nil && actionRuns.ActionWorkflow != nil {
			log.Printf("CheckInstallHealthStatus: Got %d runs for action %s", len(actionRuns.Runs), actionRuns.ActionWorkflow.Name)
			status.ActionName = actionRuns.ActionWorkflow.Name
			status.ActionID = actionRuns.ActionWorkflow.ID

			// Check most recent run status
			if len(actionRuns.Runs) > 0 {
				recentRun := actionRuns.Runs[0]
				log.Printf("CheckInstallHealthStatus: Most recent run status=%s for action %s", recentRun.Status, actionRuns.ActionWorkflow.Name)
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
