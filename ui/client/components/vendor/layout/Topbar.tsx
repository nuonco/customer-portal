import { useEffect, useMemo, useState } from "react";
import { useLocation } from "react-router";
import { Icon } from "@/components/common/Icon";
import { Link } from "@/components/common/Link";
import { Text } from "@/components/common/Text";
import { VendorUserMenu } from "@/components/vendor/layout/UserMenu";
import { VENDOR_BREADCRUMB_LABELS } from "@/const/vendor-layout";
import { useSidebar } from "@/hooks/use-sidebar";
import { useVendorAuth } from "@/hooks/use-vendor-auth";
import { getInstallLinkDetail } from "@/lib/api/vendor/get-install-links";
import { getOrgSession } from "@/lib/cookies";
import { getVendorOrgs, type TVendorOrg } from "@/lib/api/vendor/get-orgs";
import { buildVendorOrgUrl } from "@/utils/vendor-url-utils";

type TBreadcrumbItem = {
  label: string;
  href?: string;
  isCurrent?: boolean;
};

export const VendorAppTopbar = () => {
  const location = useLocation();
  const { toggleSidebar } = useSidebar();
  const { user } = useVendorAuth();
  const [orgs, setOrgs] = useState<TVendorOrg[]>([]);
  const [installLinkName, setInstallLinkName] = useState<string | null>(null);

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

  useEffect(() => {
    let isMounted = true;
    const parts = location.pathname.split("/").filter(Boolean);
    const adminParts = parts[0] === "admin" ? parts.slice(1) : parts;

    const orgId = adminParts[1];
    const section = adminParts[2];
    const linkId = adminParts[3];

    if (!orgId || section !== "install-links" || !linkId) {
      setInstallLinkName(null);
      return () => {
        isMounted = false;
      };
    }

    const loadInstallName = async () => {
      try {
        const detail = await getInstallLinkDetail(orgId, linkId);
        if (!isMounted) return;
        setInstallLinkName(
          detail.link.install?.name || detail.link.name || detail.link.app_name,
        );
      } catch {
        if (!isMounted) return;
        setInstallLinkName("Install Link");
      }
    };

    void loadInstallName();

    return () => {
      isMounted = false;
    };
  }, [location.pathname]);

  const breadcrumbs = useMemo<TBreadcrumbItem[]>(() => {
    const parts = location.pathname.split("/").filter(Boolean);
    const adminParts = parts[0] === "admin" ? parts.slice(1) : parts;

    const orgId = adminParts[1];
    const selectedOrgId = orgId ?? getOrgSession() ?? null;
    const orgName =
      orgs.find((org) => org.id === selectedOrgId)?.name ?? selectedOrgId ?? "Organization";

    const currentSection = adminParts[2] ?? adminParts[0] ?? "";
    const currentPage = VENDOR_BREADCRUMB_LABELS[currentSection] || currentSection || "Home";

    const linkId = adminParts[3];
    const isInstallLinkDetail = currentSection === "install-links" && Boolean(linkId);

    if (isInstallLinkDetail && orgId) {
      return [
        { label: orgName },
        { label: "Install Links", href: buildVendorOrgUrl(orgId, "install-links") },
        { label: installLinkName || "Install Link", isCurrent: true },
      ];
    }

    return [{ label: orgName }, { label: currentPage, isCurrent: true }];
  }, [installLinkName, location.pathname, orgs]);

  return (
    <header className="flex h-17 items-center justify-between border-b border-border-subtle bg-surface px-4 md:px-6">
      <div className="flex min-w-0 items-center gap-3">
        <button
          aria-label="Toggle sidebar"
          className="flex h-8 w-8 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-surface-elevated"
          onClick={() => toggleSidebar?.()}
          type="button"
        >
          <Icon variant="SidebarSimpleIcon" />
        </button>

        <nav className="flex min-w-0 items-center gap-2" aria-label="Breadcrumb">
          {breadcrumbs.map((crumb, index) => {
            const isLast = index === breadcrumbs.length - 1;

            return (
              <div key={`${crumb.label}-${index}`} className="flex min-w-0 items-center gap-2">
                {index > 0 ? (
                  <Icon className="text-text-muted" size={12} variant="CaretRightIcon" />
                ) : null}
                {crumb.href && !isLast ? (
                  <Link
                    href={crumb.href}
                    className="max-w-[28vw] truncate capitalize text-sm leading-6 tracking-[-0.2px]"
                  >
                    {crumb.label}
                  </Link>
                ) : (
                  <Text
                    className="max-w-[28vw] truncate capitalize"
                    variant="body"
                    weight={crumb.isCurrent || isLast ? "strong" : "normal"}
                  >
                    {crumb.label}
                  </Text>
                )}
              </div>
            );
          })}
        </nav>
      </div>

      <VendorUserMenu user={user} />
    </header>
  );
};
