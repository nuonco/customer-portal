import { Button } from "@/components/common/Button";
import { Text } from "@/components/common/Text";
import { EmptyState } from "@/components/common/EmptyState";
import { SimpleTable } from "@/components/common/SimpleTable";
import {
  type TInstallLink,
  type TInstallLinkPagination,
} from "@/lib/api/vendor/get-install-links";
import { buildVendorOrgUrl } from "@/utils/vendor-url-utils";

type TTab = "available" | "used";

interface IInstallLinksTable {
  orgId: string;
  links: TInstallLink[];
  pagination: TInstallLinkPagination | null;
  activeTab: TTab;
  onTabChange: (tab: TTab) => void;
  onPageChange: (page: number) => void;
  onCreateLink: () => void;
  navigateTo?: (href: string) => void;
}

const COLUMNS = [
  { key: "name", label: "Name" },
  { key: "status", label: "Status" },
  { key: "app", label: "App" },
  { key: "region", label: "Region" },
  { key: "created", label: "Created" },
];

const formatDate = (value: string) => {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
};

const statusClassName = (used: boolean) =>
  used
    ? "bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300"
    : "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300";

export const InstallLinksTable = ({
  orgId,
  links,
  pagination,
  activeTab,
  onTabChange,
  onPageChange,
  onCreateLink,
  navigateTo = (href) => window.location.assign(href),
}: IInstallLinksTable) => {
  const availableCount = pagination?.available_count ?? 0;
  const usedCount = pagination?.used_count ?? 0;

  const hasAnyLinks = availableCount > 0 || usedCount > 0;

  const renderEmptyState = () => {
    if (!hasAnyLinks) {
      return (
        <EmptyState
          emptyTitle="No Install Links"
          emptyMessage="Create your first link to share with customers."
          action={
            <Button variant="primary" onClick={onCreateLink}>
              Create Your First Link
            </Button>
          }
        />
      );
    }

    if (activeTab === "used") {
      return (
        <EmptyState
          emptyTitle=" No Accepted Links"
          emptyMessage=" Links will appear here once customers accept them."
        />
      );
    }

    return (
      <EmptyState
        emptyTitle="No Pending Links"
        emptyMessage="All your links have been accepted by customers."
        action={
          <Button variant="primary" onClick={onCreateLink}>
            Create New Link
          </Button>
        }
      />
    );
  };

  return (
    <>
      <div className="mb-6 border-b border-cool-grey-200 dark:border-dark-grey-600">
        <nav className="-mb-px flex space-x-8">
          <button
            className={`whitespace-nowrap border-b-2 px-1 py-2 text-sm font-medium ${
              activeTab === "available"
                ? "border-primary-600 text-primary-600"
                : "border-transparent text-cool-grey-500 hover:border-cool-grey-300 hover:text-cool-grey-700 dark:text-cool-grey-400 dark:hover:border-dark-grey-500 dark:hover:text-cool-grey-300"
            }`}
            onClick={() => onTabChange("available")}
          >
            Pending
            <span className="ml-2 rounded-full bg-cool-grey-100 px-2.5 py-0.5 text-xs font-medium text-cool-grey-900 dark:bg-dark-grey-700 dark:text-cool-grey-200">
              {availableCount}
            </span>
          </button>
          <button
            className={`whitespace-nowrap border-b-2 px-1 py-2 text-sm font-medium ${
              activeTab === "used"
                ? "border-primary-600 text-primary-600"
                : "border-transparent text-cool-grey-500 hover:border-cool-grey-300 hover:text-cool-grey-700 dark:text-cool-grey-400 dark:hover:border-dark-grey-500 dark:hover:text-cool-grey-300"
            }`}
            onClick={() => onTabChange("used")}
          >
            Accepted
            <span className="ml-2 rounded-full bg-cool-grey-100 px-2.5 py-0.5 text-xs font-medium text-cool-grey-900 dark:bg-dark-grey-700 dark:text-cool-grey-200">
              {usedCount}
            </span>
          </button>
        </nav>
      </div>

      <div className="mb-4">
        {activeTab === "used" ? (
          <>
       
            <Text variant="body" theme="neutral">
              These links have been accepted and are now owned by customers.
            </Text>
          </>
        ) : (
          <>
    
            <Text variant="body" theme="neutral">
              Share these links with customers to create their installs.
            </Text>
          </>
        )}
      </div>

      {links.length === 0 ? (
        renderEmptyState()
      ) : (
        <>
          <div className="mb-4 flex items-center justify-between">
            <span className="text-sm text-cool-grey-600 dark:text-cool-grey-400">
              {pagination?.showing_from ?? 0}-{pagination?.showing_to ?? 0} of{" "}
              {pagination?.total_count ?? 0}
            </span>
          </div>

          <SimpleTable headers={COLUMNS}>
            {links.map((link) => {
              const href = buildVendorOrgUrl(orgId, `install-links/${link.id}`);

              return (
                <tr
                  key={link.id}
                  role="link"
                  tabIndex={0}
                  data-testid={`install-link-row-${link.id}`}
                  className="cursor-pointer hover:bg-cool-grey-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-inset dark:hover:bg-dark-grey-800"
                  onClick={() => navigateTo(href)}
                  onKeyDown={(event) => {
                    if (event.key !== "Enter" && event.key !== " ") return;
                    event.preventDefault();
                    navigateTo(href);
                  }}
                >
                  <td className="px-4 py-3 whitespace-nowrap text-sm font-medium text-primary-600 dark:text-primary-400">
                    {link.install?.name || link.app_name}
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <span
                      className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${statusClassName(link.used)}`}
                    >
                      {link.used ? "Accepted" : "Pending"}
                    </span>
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-cool-grey-900 dark:text-white">
                    {link.app_name}
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-cool-grey-600 dark:text-cool-grey-400">
                    {link.install?.region || "—"}
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-cool-grey-500 dark:text-cool-grey-400">
                    {formatDate(link.created_at)}
                  </td>
                </tr>
              );
            })}
          </SimpleTable>

          {(pagination?.total_pages ?? 1) > 1 ? (
            <div className="mt-4 flex items-center justify-between">
              <Button
                variant="secondary"
                disabled={!pagination?.has_previous}
                onClick={() => onPageChange(pagination?.previous_page ?? 1)}
              >
                Previous
              </Button>
              <Text variant="label" theme="neutral">
                Page {pagination?.current_page ?? 1} of{" "}
                {pagination?.total_pages ?? 1}
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
      )}
    </>
  );
};
