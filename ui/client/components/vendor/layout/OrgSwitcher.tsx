import { useEffect, useMemo, useState } from "react";
import { ConnectOrgModal } from "@/components/vendor/org/ConnectOrgModal";
import { getVendorOrgs, type TVendorOrg } from "@/lib/api/vendor/get-orgs";
import { getOrgSession, setOrgSession } from "@/lib/cookies";
import { Icon } from "@/components/common/Icon";
import { cn } from "@/utils/classnames";
import { buildVendorOrgUrl } from "@/utils/vendor-url-utils";

interface IOrgSwitcherProps {
  isSidebarOpen: boolean;
}

export const OrgSwitcher = ({ isSidebarOpen }: IOrgSwitcherProps) => {
  const [orgs, setOrgs] = useState<TVendorOrg[]>([]);
  const [isOrgDropdownOpen, setIsOrgDropdownOpen] = useState(false);
  const [isCreateOrgOpen, setIsCreateOrgOpen] = useState(false);

  useEffect(() => {
    let isMounted = true;

    const loadOrgs = async () => {
      try {
        const results = await getVendorOrgs();
        if (isMounted) {
          setOrgs(results);
        }
      } catch {
        if (isMounted) {
          setOrgs([]);
        }
      }
    };

    void loadOrgs();

    return () => {
      isMounted = false;
    };
  }, []);

  const selectedOrg = useMemo(() => {
    const selectedOrgId = getOrgSession();
    if (!selectedOrgId) {
      return orgs[0] ?? null;
    }
    return orgs.find((org) => org.id === selectedOrgId) ?? orgs[0] ?? null;
  }, [orgs]);

  const handleSelectOrg = (org: TVendorOrg) => {
    setOrgSession(org.id);
    setIsOrgDropdownOpen(false);
    window.location.assign(buildVendorOrgUrl(org.id, "accounts"));
  };

  const openCreateOrgModal = () => {
    setIsOrgDropdownOpen(false);
    setIsCreateOrgOpen(true);
  };

  const closeCreateOrgModal = () => {
    setIsCreateOrgOpen(false);
  };

  useEffect(() => {
    setIsOrgDropdownOpen(false);
  }, [isSidebarOpen]);

  return (
    <div className="relative mt-3 mb-4">
      <button
        type="button"
        aria-label="Select organization"
        className="flex w-full items-center gap-2 rounded-lg border border-border-subtle bg-surface px-2.5 py-2 text-left shadow-sm transition-colors hover:bg-surface-elevated cursor-pointer"
        onClick={() => setIsOrgDropdownOpen((prev) => !prev)}
      >
        <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-surface-elevated text-[11px] font-semibold text-text-primary">
          {selectedOrg ? (
            selectedOrg.name.slice(0, 2).toUpperCase()
          ) : (
            <Icon size={16} variant="BuildingsIcon" />
          )}
        </span>

        {selectedOrg ? (
          <span className={cn("min-w-0 flex-1", !isSidebarOpen && "md:hidden")}>
            <span className="block truncate text-xs font-semibold text-text-primary">
              {selectedOrg.name}
            </span>
            <span className="mt-0.5 flex items-center gap-1 text-[11px] text-text-muted">
              <span className="h-1.5 w-1.5 rounded-full bg-green-500" />
              <span>Active</span>
            </span>
          </span>
        ) : (
          <span className={cn("min-w-0 flex-1 truncate text-sm text-text-muted", !isSidebarOpen && "md:hidden")}>
            Select Organization
          </span>
        )}

        <Icon
          className={cn("shrink-0 text-text-muted", !isSidebarOpen && "md:hidden")}
          size={14}
          variant="CaretUpDownIcon"
        />
      </button>

      {isOrgDropdownOpen ? (
        <div
          className={cn(
            "absolute left-0 z-20 mt-2 rounded-md border border-border-subtle bg-surface p-2 shadow-lg",
            isSidebarOpen ? "w-[260px]" : "w-[300px]",
          )}
        >
          {selectedOrg ? (
            <div className="mb-2 rounded border border-border-subtle bg-surface-muted px-3 py-2">
              <div className="mb-1 text-[11px] font-medium text-text-muted">Org status</div>
              <div className="flex items-center gap-1.5 text-xs text-green-600 dark:text-green-400">
                <span className="h-1.5 w-1.5 rounded-full bg-green-500" />
                <span>Connected</span>
              </div>
            </div>
          ) : null}

          <div className="max-h-52 overflow-y-auto">
            {orgs.length > 0 ? (
              orgs.map((org) => (
                <button
                  key={org.id}
                  type="button"
                  className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm text-text-primary hover:bg-surface-elevated"
                  onClick={() => handleSelectOrg(org)}
                >
                  <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded bg-surface-elevated text-[10px] font-semibold">
                    {org.name.slice(0, 2).toUpperCase()}
                  </span>
                  <span className="truncate">{org.name}</span>
                </button>
              ))
            ) : (
              <div className="px-2 py-2 text-xs text-text-muted">
                No organizations connected yet.
              </div>
            )}
          </div>

          <div className="mt-2 border-t border-border-subtle pt-2">
            <button
              type="button"
              className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm text-text-primary hover:bg-surface-elevated cursor-pointer"
              onClick={openCreateOrgModal}
            >
              <Icon size={14} variant="PlusIcon" />
              <span>Connect Org</span>
            </button>
          </div>
        </div>
      ) : null}

      <ConnectOrgModal
        isOpen={isCreateOrgOpen}
        onClose={closeCreateOrgModal}
      />
    </div>
  );
};