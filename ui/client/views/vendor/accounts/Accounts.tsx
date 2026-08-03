import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router";
import { PageHeader } from "@/components/vendor/layout/PageHeader";
import { Text } from "@/components/common/Text";
import { Message } from "@/components/common/Message";
import { Input } from "@/components/common/form/Input";
import { SimpleTable } from "@/components/common/SimpleTable";
import { EmptyState } from "@/components/common/EmptyState";
import { getAccounts, type TAccount } from "@/lib/api/vendor/get-accounts";
import { buildVendorOrgUrl } from "@/utils/vendor-url-utils";

// TODO: revisit to test when the user is added to groups

const COLUMNS = [
  { key: "name", label: "User Group" },
  { key: "members", label: "Members" },
  { key: "installs", label: "Installs" },
  { key: "created", label: "Created" },
];

export const AccountsView = () => {
  const { orgId } = useParams<{ orgId: string }>();

  const [accounts, setAccounts] = useState<TAccount[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [search, setSearch] = useState("");

  const searchTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const hasLoadedInitiallyRef = useRef(false);

  const fetchAccounts = useCallback(
    async (q: string) => {
      if (!orgId) return;
      setIsLoading(true);
      setLoadError(null);
      try {
        const data = await getAccounts(orgId, { q });
        setAccounts(data.accounts ?? []);
      } catch {
        setLoadError("Unable to load user groups right now.");
        setAccounts([]);
      } finally {
        setIsLoading(false);
      }
    },
    [orgId],
  );

  useEffect(() => {
    void fetchAccounts("");
    hasLoadedInitiallyRef.current = true;
  }, [fetchAccounts]);

  useEffect(() => {
    if (!hasLoadedInitiallyRef.current) return;
    if (search === "") return;
    if (searchTimerRef.current) clearTimeout(searchTimerRef.current);
    searchTimerRef.current = setTimeout(() => {
      void fetchAccounts(search);
    }, 300);
    return () => {
      if (searchTimerRef.current) clearTimeout(searchTimerRef.current);
    };
  }, [search, fetchAccounts]);

  const content = (() => {
    if (isLoading) {
      return (
        <div className="rounded-lg border border-cool-grey-300 dark:border-dark-grey-500 bg-white dark:bg-dark-grey-900 p-12 text-center">
          <Text variant="body" theme="neutral">
            Loading user groups...
          </Text>
        </div>
      );
    }

    if (loadError) {
      return (
        <Message theme="warning">{loadError}</Message>
      );
    }

    if (accounts.length === 0 && search === "") {
      return (
        <div className="rounded-lg border border-cool-grey-300 dark:border-dark-grey-500 bg-white dark:bg-dark-grey-900 p-12 shadow-md">
          <EmptyState
            emptyTitle="No User Groups Yet"
            emptyMessage="Customer user groups will appear here once customers have signed up and been assigned to groups."
          />
        </div>
      );
    }

    return (
      <SimpleTable headers={COLUMNS}>
        {accounts.length === 0 ? (
          <tr>
            <td
              colSpan={COLUMNS.length}
              className="px-6 py-8 text-center text-sm text-cool-grey-500 dark:text-cool-grey-400"
            >
              No user groups found matching &ldquo;{search}&rdquo;
            </td>
          </tr>
        ) : (
          accounts.map((account) => (
            <tr
              key={account.id}
              className="hover:bg-cool-grey-50 dark:hover:bg-dark-grey-800"
            >
              <td className="px-4 py-3 whitespace-nowrap">
                <Link
                  to={buildVendorOrgUrl(orgId, `accounts/${account.id}/members`)}
                  className="text-sm font-medium text-primary-600 dark:text-primary-400 hover:underline"
                >
                  {account.name}
                </Link>
              </td>
              <td className="px-4 py-3 whitespace-nowrap text-sm text-cool-grey-700 dark:text-cool-grey-300">
                {account.member_count}
              </td>
              <td className="px-4 py-3 whitespace-nowrap text-sm text-cool-grey-700 dark:text-cool-grey-300">
                {account.install_count}
              </td>
              <td className="px-4 py-3 whitespace-nowrap text-sm text-cool-grey-500 dark:text-cool-grey-400">
                {new Date(account.created_at).toLocaleDateString()}
              </td>
            </tr>
          ))
        )}
      </SimpleTable>
    );
  })();

  return (
    <section className="overflow-auto">
      <PageHeader
        title="User Groups"
        subtitle="View customer user groups and their members."
      />

      <div className="flex flex-col gap-4">
        {(accounts.length > 0 || search !== "") && !isLoading && !loadError ? (
          <Input
            id="accounts-search"
            labelProps={{ labelText: "Search" }}
            placeholder="Search by user group name..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        ) : null}
        {content}
      </div>
    </section>
  );
};
