import { Button } from '@/components/common/Button'
import { CloudPlatform } from '@/components/common/CloudPlatform'
import { EmptyState } from '@/components/common/EmptyState'
import { Icon } from '@/components/common/Icon'
import { SimpleTable } from '@/components/common/SimpleTable'
import type { TCloudPlatform } from '@/types'
import { type TAdminInstall } from '@/lib/api/vendor/get-installs'
import { buildVendorOrgUrl } from '@/utils/vendor-url-utils'

const COLUMNS = [
  { key: 'name', label: 'Name' },
  { key: 'owner', label: 'Owner' },
  { key: 'app', label: 'App' },
  { key: 'platform', label: 'Platform' },
  { key: 'created', label: 'Created' },
  { key: 'forget', label: 'Forget', align: 'center' as const },
  { key: 'portal', label: 'Portal', align: 'center' as const },
  { key: 'dash', label: 'Dash', align: 'center' as const },
]

interface IInstallsTable {
  orgId: string
  installs: TAdminInstall[]
  hasFilters: boolean
  onForget: (install: TAdminInstall) => void
  onImport: () => void
}

export const InstallsTable = ({
  orgId,
  installs,
  hasFilters,
  onForget,
  onImport,
}: IInstallsTable) => {
  if (installs.length === 0 && !hasFilters) {
    return (
      <div className="rounded-lg border border-cool-grey-300 dark:border-dark-grey-500 bg-white dark:bg-dark-grey-900 p-12 shadow-md">
        <EmptyState
          emptyTitle="No Installs Yet"
          emptyMessage="No installs are tracked in the portal yet. Customers create installs via install links, or you can import an existing install below."
          action={
            <Button variant="primary" onClick={onImport}>
              Import Install
            </Button>
          }
        />
      </div>
    )
  }

  return (
    <SimpleTable headers={COLUMNS}>
      {installs.length === 0 ? (
        <tr>
          <td
            colSpan={COLUMNS.length}
            className="px-6 py-8 text-center text-sm text-cool-grey-500 dark:text-cool-grey-400"
          >
            No installs found matching your filters.
          </td>
        </tr>
      ) : (
        installs.map((install) => (
          <tr
            key={install.ID}
            className="hover:bg-cool-grey-50 dark:hover:bg-dark-grey-800"
          >
            {/* Name */}
            <td className="px-4 py-3 whitespace-nowrap">
              <div className="text-sm font-medium text-cool-grey-900 dark:text-white">
                {install.Name}
                {install.APIDeleted ? (
                  <span className="ml-2 inline-flex items-center rounded px-2 py-0.5 text-xs font-medium bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">
                    Removed from Nuon
                  </span>
                ) : null}
              </div>
              <div className="font-mono text-xs text-cool-grey-500 dark:text-cool-grey-400">
                {install.ID}
              </div>
            </td>
            {/* Owner */}
            <td className="px-4 py-3 whitespace-nowrap">
              <a
                href={buildVendorOrgUrl(orgId, `customers/${install.CustomerID}`)}
                className="block text-sm font-medium text-primary-600 dark:text-primary-400 hover:underline"
              >
                {install.CustomerEmail}
              </a>
              {install.CustomerName && install.CustomerName !== install.CustomerEmail ? (
                <div className="text-xs text-cool-grey-500 dark:text-cool-grey-400">
                  {install.CustomerName}
                </div>
              ) : null}
            </td>
            {/* App */}
            <td className="px-4 py-3 whitespace-nowrap">
              {install.AppID ? (
                <a
                  href={buildVendorOrgUrl(orgId, `apps/${install.AppID}/inputs`)}
                  className="text-sm text-primary-600 dark:text-primary-400 hover:underline"
                >
                  {install.AppName}
                </a>
              ) : (
                <span className="text-sm text-cool-grey-400 dark:text-cool-grey-600">
                  unknown
                </span>
              )}
            </td>
            {/* Platform */}
            <td className="px-4 py-3 whitespace-nowrap">
              <CloudPlatform
                platform={install.Platform as TCloudPlatform}
                displayVariant="abbr"
              />
            </td>
            {/* Created */}
            <td className="px-4 py-3 whitespace-nowrap text-sm text-cool-grey-900 dark:text-white">
              {install.CreatedAt}
            </td>
            {/* Forget */}
            <td className="px-3 py-3 whitespace-nowrap text-center">
              <Button
                variant="danger"
                size="sm"
                onClick={() => onForget(install)}
                title="Forget install (removes from portal only)"
              >
                Forget
              </Button>
            </td>
            {/* Portal */}
            <td className="px-3 py-3 whitespace-nowrap text-center">
              <a
                href={`/installs/${install.ID}`}
                target="_blank"
                rel="noopener noreferrer"
                title="View in Customer Portal"
                className="inline-flex items-center text-primary-600 dark:text-primary-400 hover:text-primary-900 dark:hover:text-primary-300"
              >
                <Icon variant="EyeIcon" />
              </a>
            </td>
            {/* Dash */}
            <td className="px-3 py-3 whitespace-nowrap text-center">
              <a
                href={`https://app.nuon.co/${install.NuonOrgID}/installs/${install.NuonInstallID}`}
                target="_blank"
                rel="noopener noreferrer"
                title="View in Nuon Dashboard"
                className="inline-flex items-center text-primary-600 dark:text-primary-400 hover:text-primary-900 dark:hover:text-primary-300"
              >
                <Icon variant="ArrowSquareOutIcon" />
              </a>
            </td>
          </tr>
        ))
      )}
    </SimpleTable>
  )
}
