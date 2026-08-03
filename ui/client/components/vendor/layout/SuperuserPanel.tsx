import { useMemo, useState } from "react";
import { Button } from "@/components/common/Button";
import { Message } from "@/components/common/Message";
import { Text } from "@/components/common/Text";
import { PanelBase } from "@/components/surfaces/Panel";
import type { IPanel } from "@/components/surfaces/Panel";
import {
  type TSuperuserOrg,
  getSuperuserOrg,
  joinSuperuserOrg,
  leaveSuperuserOrg,
  searchSuperuserOrgs,
} from "@/lib/api/vendor/superuser";

type TSuperuserPanelProps = Pick<IPanel, "isVisible" | "panelId" | "panelKey">;

export const VendorSuperuserPanel = ({
  isVisible,
  panelId,
  panelKey,
}: TSuperuserPanelProps) => {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<TSuperuserOrg[]>([]);
  const [selectedOrg, setSelectedOrg] = useState<TSuperuserOrg | null>(null);
  const [isSearching, setIsSearching] = useState(false);
  const [isLoadingDetail, setIsLoadingDetail] = useState(false);
  const [isMutating, setIsMutating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const showEmptyResult = useMemo(
    () => query.trim().length > 0 && !isSearching && results.length === 0,
    [isSearching, query, results.length],
  );

  const handleSearch = async (value: string) => {
    const trimmed = value.trim();
    setQuery(value);
    setSelectedOrg(null);
    setError(null);

    if (!trimmed) {
      setResults([]);
      return;
    }

    setIsSearching(true);
    try {
      const orgs = await searchSuperuserOrgs(trimmed);
      setResults(orgs);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to search organizations",
      );
      setResults([]);
    } finally {
      setIsSearching(false);
    }
  };

  const handleSelectOrg = async (orgId: string) => {
    setError(null);
    setIsLoadingDetail(true);
    try {
      const org = await getSuperuserOrg(orgId);
      setSelectedOrg(org);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to load organization details",
      );
    } finally {
      setIsLoadingDetail(false);
    }
  };

  const handleMembershipAction = async () => {
    if (!selectedOrg) return;

    setError(null);
    setIsMutating(true);
    try {
      const updated = selectedOrg.is_member
        ? await leaveSuperuserOrg(selectedOrg.id)
        : await joinSuperuserOrg(selectedOrg.id);
      setSelectedOrg(updated);
      setResults((prev) =>
        prev.map((org) => (org.id === updated.id ? updated : org)),
      );

      // Membership changes affect org-scoped navigation state (e.g. org dropdown),
      // so refresh after a successful mutation to pull a fresh server-derived view.
      window.location.reload();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to update membership",
      );
    } finally {
      setIsMutating(false);
    }
  };

  return (
    <PanelBase
      heading="Superuser"
      size="default"
      isVisible={isVisible}
      panelId={panelId}
      panelKey={panelKey}
    >
      <Text as="p" variant="body" theme="neutral">
        Search and manage organization membership.
      </Text>

      <div className="space-y-2">
        <input
          type="text"
          value={query}
          onChange={(e) => {
            void handleSearch(e.target.value);
          }}
          placeholder="Search by name, subdomain, or org ID..."
          className="w-full rounded-md border border-border-subtle bg-surface px-3 py-2 text-sm text-text-primary outline-none focus:border-primary-500 focus:ring-1 focus:ring-primary-500"
        />
      </div>

      {isSearching ? <Text variant="subtext">Searching...</Text> : null}

      {showEmptyResult ? <Text variant="subtext">No orgs found.</Text> : null}

      {results.length > 0 ? (
        <div className="space-y-1">
          {results.map((org) => (
            <button
              key={org.id}
              type="button"
              onClick={() => {
                void handleSelectOrg(org.id);
              }}
              className="w-full rounded-md px-3 py-2 text-left transition-colors hover:bg-surface-elevated"
            >
              <div className="text-sm font-medium text-text-primary">
                {org.name || "Unnamed Org"}
              </div>
              <div className="mt-0.5 flex gap-3 text-xs text-text-muted">
                {org.subdomain ? <span>{org.subdomain}</span> : null}
                {org.nuon_org_id ? (
                  <span className="font-mono">{org.nuon_org_id}</span>
                ) : null}
              </div>
            </button>
          ))}
        </div>
      ) : null}

      {isLoadingDetail ? (
        <Text variant="subtext">Loading organization...</Text>
      ) : null}

      {selectedOrg ? (
        <div className="rounded-md border border-border-subtle p-4">
          <div className="mb-2 text-sm font-semibold text-text-primary">
            {selectedOrg.name || "Unnamed Org"}
          </div>
          <div className="space-y-1 text-xs text-text-muted">
            <div className="flex gap-2">
              <span className="w-20 font-medium">ID:</span>
              <span className="font-mono">{selectedOrg.id}</span>
            </div>
            {selectedOrg.subdomain ? (
              <div className="flex gap-2">
                <span className="w-20 font-medium">Subdomain:</span>
                <span>{selectedOrg.subdomain}</span>
              </div>
            ) : null}
            {selectedOrg.nuon_org_id ? (
              <div className="flex gap-2">
                <span className="w-20 font-medium">Nuon Org:</span>
                <span className="font-mono">{selectedOrg.nuon_org_id}</span>
              </div>
            ) : null}
          </div>

          <div className="mt-4">
            <Button
              variant={selectedOrg.is_member ? "danger" : "primary"}
              onClick={handleMembershipAction}
              disabled={isMutating}
            >
              {isMutating
                ? selectedOrg.is_member
                  ? "Leaving..."
                  : "Joining..."
                : selectedOrg.is_member
                  ? "Leave Org"
                  : "Join Org"}
            </Button>
          </div>
        </div>
      ) : null}

      {error ? (
        <Message className="rounded-md px-3 py-2">
          {error}
        </Message>
      ) : null}
    </PanelBase>
  );
};
