import { Button } from "@/components/common/Button";
import { CloudPlatform } from "@/components/common/CloudPlatform";
import { EmptyState } from "@/components/common/EmptyState";
import { Icon } from "@/components/common/Icon";
import { SimpleTable } from "@/components/common/SimpleTable";
import { Text } from "@/components/common/Text";
import type {
  TAppCatalogItem,
  TAppCatalogPagination,
  TAppCatalogStatus,
} from "@/lib/api/vendor/get-app-catalog";
import { buildVendorOrgUrl } from "@/utils/vendor-url-utils";

const COLUMNS = [
  { key: "move", label: "Order", align: "center" as const },
  { key: "logo", label: "Logo" },
  { key: "name", label: "Name" },
  { key: "platform", label: "Platform" },
  { key: "published", label: "Published" },
  { key: "status", label: "Status" },
];

const STATUS_OPTIONS: { value: TAppCatalogStatus; label: string }[] = [
  { value: "unpublished", label: "Unpublished" },
  { value: "coming_soon", label: "Coming Soon" },
  { value: "published", label: "Published" },
];

interface IAppCatalogTable {
  orgId: string;
  apps: TAppCatalogItem[];
  pagination: TAppCatalogPagination | null;
  hasChanges: boolean;
  isSaving: boolean;
  onMove: (index: number, direction: "up" | "down") => void;
  onStatusChange: (appId: string, status: TAppCatalogStatus) => void;
  onSave: () => void;
  onForgetDeletedApp: (app: TAppCatalogItem) => void;
  onPageChange: (page: number) => void;
}

const StatusBadge = ({ deleted }: { deleted: boolean }) => {
  if (deleted) {
    return (
      <span className="inline-flex rounded px-2 py-0.5 text-xs font-medium bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300">
        Deleted from API
      </span>
    );
  }

  return (
    <span className="inline-flex rounded px-2 py-0.5 text-xs font-medium bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300">
      Active
    </span>
  );
};

export const AppCatalogTable = ({
  orgId,
  apps,
  pagination,
  hasChanges,
  isSaving,
  onMove,
  onStatusChange,
  onSave,
  onForgetDeletedApp,
  onPageChange,
}: IAppCatalogTable) => {
  if (apps.length === 0) {
    return (
      <div className="rounded-lg border border-cool-grey-300 dark:border-dark-grey-500 bg-white dark:bg-dark-grey-900 p-12 shadow-md">
        <EmptyState
          emptyTitle="No Apps Found"
          emptyMessage="No apps are currently available in this organization."
        />
      </div>
    );
  }

  return (
    <>
      <SimpleTable headers={COLUMNS}>
        {apps.map((app, index) => {
          const canMoveUp = index > 0;
          const canMoveDown = index < apps.length - 1;
          const appPath = app.deleted
            ? ""
            : buildVendorOrgUrl(orgId, `apps/${app.id}/overview`);

          return (
            <tr
              key={app.id}
              className={app.deleted ? "bg-amber-50 dark:bg-amber-900/20" : "hover:bg-cool-grey-50 dark:hover:bg-dark-grey-800"}
            >
              <td className="px-3 py-3 text-center align-middle">
                <div className="inline-flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="xs"
                    disabled={!canMoveUp}
                    onClick={() => onMove(index, "up")}
                    title="Move up"
                  >
                    <Icon variant="ArrowUpIcon" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="xs"
                    disabled={!canMoveDown}
                    onClick={() => onMove(index, "down")}
                    title="Move down"
                  >
                    <Icon variant="ArrowDownIcon" />
                  </Button>
                </div>
              </td>
              <td className="px-4 py-3 align-middle">
                {app.logo_light_base64 ? (
                  app.logo_dark_base64 ? (
                    <>
                      <img
                        src={app.logo_light_base64}
                        className="h-8 w-8 rounded object-contain dark:hidden"
                        alt=""
                      />
                      <img
                        src={app.logo_dark_base64}
                        className="hidden h-8 w-8 rounded object-contain dark:block"
                        alt=""
                      />
                    </>
                  ) : (
                    <img
                      src={app.logo_light_base64}
                      className="h-8 w-8 rounded object-contain"
                      alt=""
                    />
                  )
                ) : app.logo_dark_base64 ? (
                  <img
                    src={app.logo_dark_base64}
                    className="h-8 w-8 rounded object-contain"
                    alt=""
                  />
                ) : (
                  <span className="text-cool-grey-400 dark:text-cool-grey-600">-</span>
                )}
              </td>
              <td className="px-4 py-3 align-middle">
                {appPath ? (
                  <a
                    href={appPath}
                    className="text-sm font-medium text-primary-600 dark:text-primary-400 hover:underline"
                  >
                    {app.name}
                  </a>
                ) : (
                  <span className="text-sm font-medium text-cool-grey-900 dark:text-white">
                    {app.name}
                  </span>
                )}
                <div className="font-mono text-xs text-cool-grey-500 dark:text-cool-grey-400">
                  {app.id}
                </div>
              </td>
              <td className="px-4 py-3 align-middle">
                {app.deleted ? (
                  <span className="text-sm text-cool-grey-400 dark:text-cool-grey-600">-</span>
                ) : (
                  <CloudPlatform platform={app.platform as "aws" | "azure" | "gcp"} displayVariant="abbr" />
                )}
              </td>
              <td className="px-4 py-3 align-middle">
                {app.deleted ? (
                  <Button
                    variant="danger"
                    size="sm"
                    onClick={() => onForgetDeletedApp(app)}
                  >
                    Remove
                  </Button>
                ) : (
                  <select
                    className="text-sm rounded-md border border-cool-grey-300 dark:border-dark-grey-500 bg-white dark:bg-dark-grey-800 text-cool-grey-900 dark:text-white px-2 py-1"
                    value={app.status}
                    onChange={(event) =>
                      onStatusChange(app.id, event.target.value as TAppCatalogStatus)
                    }
                  >
                    {STATUS_OPTIONS.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                )}
              </td>
              <td className="px-4 py-3 align-middle">
                <StatusBadge deleted={app.deleted} />
              </td>
            </tr>
          );
        })}
      </SimpleTable>

      <div className="mt-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <Text variant="subtext" theme="neutral">
          Reorder apps and update publish states, then save changes.
        </Text>
        <Button variant="primary" onClick={onSave} disabled={!hasChanges || isSaving}>
          {isSaving ? "Saving..." : "Save"}
        </Button>
      </div>

      {pagination?.has_previous || pagination?.has_next ? (
        <div className="mt-6 flex items-center justify-between">
          <Button
            variant="secondary"
            disabled={!pagination?.has_previous}
            onClick={() => onPageChange(pagination?.previous_page ?? 1)}
          >
            Previous
          </Button>
          <Text variant="label" theme="neutral">
            Page {pagination?.current_page ?? 1}
          </Text>
          <Button
            variant="secondary"
            disabled={!pagination?.has_next}
            onClick={() => onPageChange(pagination?.next_page ?? 1)}
          >
            Next
          </Button>
        </div>
      ) : null}
    </>
  );
};