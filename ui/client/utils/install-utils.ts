type TTitleMap = Record<string, string>

export type TStatusBadgeTheme = 'neutral' | 'success' | 'warn' | 'error' | 'info'

const RUNNER_STATUS_TITLES: TTitleMap = {
  active: 'Runner is healthy',
  error: 'Runner is unhealthy',
  pending: 'Runner is pending',
  provisioning: 'Runner is provisioning',
  deprovisioning: 'Runner is deprovisioning',
  deprovisioned: 'Runner is deprovisioned',
  reprovisioning: 'Runner is reprovisioning',
  offline: 'Runner is offline',
  'awaiting-install-stack-run': 'Runner is awaiting install stack run',
  unknown: 'Runner status is unknown',
}

const SANDBOX_STATUS_TITLES: TTitleMap = {
  active: 'Sandbox is provisioned',
  error: 'Sandbox has an error',
  queued: 'Sandbox is queued',
  provisioning: 'Sandbox is provisioning',
  deprovisioning: 'Sandbox is deprovisioning',
  deprovisioned: 'Sandbox is deprovisioned',
  reprovisioning: 'Sandbox is reprovisioning',
  access_error: 'Sandbox has an access error',
  deleted: 'Sandbox has been deleted',
  delete_failed: 'Sandbox deletion failed',
  empty: 'Sandbox is empty',
  unknown: 'Sandbox status is unknown',
}

const COMPONENTS_STATUS_TITLES: TTitleMap = {
  active: 'Components are deployed',
  inactive: 'Components are inactive',
  error: 'Component has an error',
  noop: 'Deployment had no changes',
  planning: 'Deployment is planning',
  syncing: 'Deployment is syncing',
  executing: 'Deployment is executing',
  cancelled: 'Deployment was cancelled',
  pending: 'Deployment is pending',
  queued: 'Deployment is queued',
  'pending-approval': 'Deployment is pending approval',
  'approval-denied': 'Deployment approval was denied',
  unknown: 'Deployment status is unknown',
}

// Helper for fallback
function getStatusTitle(
  map: TTitleMap,
  status: string,
  fallback: string
): string {
  return map[status] ?? fallback
}

export function getInstallRunnerStatusTitle(status: string): string {
  return getStatusTitle(
    RUNNER_STATUS_TITLES,
    status,
    RUNNER_STATUS_TITLES.unknown
  )
}

export function getInstallSandboxStatusTitle(status: string): string {
  return getStatusTitle(
    SANDBOX_STATUS_TITLES,
    status,
    SANDBOX_STATUS_TITLES.unknown
  )
}

export function getInstallComponentsStatusTitle(status: string): string {
  return getStatusTitle(
    COMPONENTS_STATUS_TITLES,
    status,
    COMPONENTS_STATUS_TITLES.unknown
  )
}

export function getInstallStatusTitle(
  statusKey: string,
  status: string
): string {
  switch (statusKey) {
    case 'runner_status':
      return getInstallRunnerStatusTitle(status)
    case 'sandbox_status':
      return getInstallSandboxStatusTitle(status)
    case 'composite_component_status':
      return getInstallComponentsStatusTitle(status)
    default:
      return 'Waiting on status'
  }
}

export function formatInstallDate(timestamp: string): string {
  const date = new Date(timestamp)
  if (Number.isNaN(date.getTime())) {
    return 'Unknown'
  }

  return date.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

export function formatInstallStatus(status: string): string {
  if (!status) {
    return 'Unknown'
  }

  return status
    .split('_')
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

export function getInstallStatusBadgeTheme(status: string): TStatusBadgeTheme {
  const normalized = status.toLowerCase()

  if (
    ['ready', 'success', 'completed', 'active', 'pass', 'running'].includes(
      normalized
    )
  ) {
    return 'success'
  }

  if (
    ['warn', 'warning', 'approval-awaiting', 'pending'].includes(normalized)
  ) {
    return 'warn'
  }

  if (['deny', 'denied', 'error', 'failed', 'cancelled'].includes(normalized)) {
    return 'error'
  }

  if (['in-progress', 'deploying', 'upgrading'].includes(normalized)) {
    return 'info'
  }

  return 'neutral'
}
