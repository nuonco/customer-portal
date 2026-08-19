import { getRuntimeConfig } from "@/lib/runtime-config";
import { useEffect, useMemo, useState } from "react";
import { useLocation } from "react-router";
import { Icon } from "@/components/common/Icon";
import { Link } from "@/components/common/Link";
import { SidebarLogo } from "@/components/common/Logo/Logo";
import { OrgSwitcher } from "@/components/vendor/layout/OrgSwitcher";
import { Text } from "@/components/common/Text";
import { MAIN_NAV_SECTIONS, type IVendorNavItem } from "@/const/vendor-layout";
import { useSidebar } from "@/hooks/use-sidebar";
import { getOrgSession } from "@/lib/cookies";
import { getVendorOrgs, type TVendorOrg } from "@/lib/api/vendor/get-orgs";
import { cn } from "@/utils/classnames";

export const VendorAppSidebar = () => {
  const location = useLocation();
  const { isSidebarOpen } = useSidebar();
  const [orgs, setOrgs] = useState<TVendorOrg[]>([]);

  const isOrgLandingRoute = /^\/admin\/orgs\/?$/.test(location.pathname);
  const pathOrgId = location.pathname.match(/^\/admin\/orgs\/([^/]+)/)?.[1] ?? null;
  const selectedOrgId = pathOrgId ?? (isOrgLandingRoute ? null : getOrgSession() ?? null);
  const selectedOrgBasePath = selectedOrgId ? `/admin/orgs/${selectedOrgId}` : null;
  const runtimeConfig = getRuntimeConfig();
  const portalBaseDomain =
    runtimeConfig.subdomainBaseDomain || window.location.host;
  const fallbackCustomerSubdomain = runtimeConfig.customerSubdomain || undefined;

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

  const customerPortalUrl = useMemo(() => {
    if (!selectedOrgId) {
      return null;
    }

    const selectedOrg = orgs.find((org) => org.id === selectedOrgId);
    const subdomain = selectedOrg?.subdomain || fallbackCustomerSubdomain;
    if (!subdomain) {
      return null;
    }

    return `${window.location.protocol}//${subdomain}.${portalBaseDomain}/`;
  }, [fallbackCustomerSubdomain, orgs, portalBaseDomain, selectedOrgId]);

  const buildItemHref = (item: IVendorNavItem) => {
    if (item.kind === "view-org") {
      return selectedOrgId ? `https://app.nuon.co/${selectedOrgId}` : "#";
    }

    if (item.kind === "view-portal") {
      return customerPortalUrl ?? "#";
    }

    if (item.isExternal) {
      return item.href ?? "#";
    }
    if (!selectedOrgBasePath || !item.suffix) {
      return "#";
    }
    return `${selectedOrgBasePath}/${item.suffix}`;
  };

  const isItemDisabled = (item: IVendorNavItem) => {
    if (!selectedOrgBasePath) {
      return !item.allowWithoutOrg;
    }

    if (item.kind === "view-org") {
      return !selectedOrgBasePath;
    }

    if (item.kind === "view-portal") {
      return !customerPortalUrl;
    }

    return !item.isExternal && (!selectedOrgBasePath || !item.suffix);
  };

  const isItemActive = (item: IVendorNavItem) => {
    if (item.isExternal || isItemDisabled(item)) {
      return false;
    }

    const itemHref = buildItemHref(item);

    if (item.label === "Groups") {
      return /\/accounts(\/|$)/.test(location.pathname);
    }

    if (item.label === "Customer Portal") {
      return /\/portal(\/|$)/.test(location.pathname);
    }

    return (
      location.pathname === itemHref ||
      location.pathname.startsWith(`${itemHref}/`)
    );
  };

  return (
    <aside
      className={cn(
        "border-r border-border-subtle bg-surface transition-all duration-200",
        "hidden md:flex md:h-screen md:flex-col",
        isSidebarOpen ? "md:w-[272px]" : "md:w-[88px]",
      )}
      id="main-sidebar"
    >
      <header className="flex h-17 items-center px-5">
        <SidebarLogo />
      </header>

      <div className="flex-1 overflow-y-auto px-4 pb-4">
        <OrgSwitcher isSidebarOpen={isSidebarOpen} />

        <div className="mt-2 space-y-4">
          {MAIN_NAV_SECTIONS.map((section) => (
            <div key={section.label} className="flex flex-col gap-2">
              <Text
                className={cn("px-3", !isSidebarOpen && "md:sr-only")}
                theme="neutral"
                variant="label"
              >
                {section.label}
              </Text>
              <nav className="space-y-1">
                {section.items.map((item) => {
                  const itemHref = buildItemHref(item);
                  const itemDisabled = isItemDisabled(item);

                  if (itemDisabled) {
                    return (
                      <span
                        key={item.label}
                        className="flex h-9 w-full items-center gap-4 rounded-md px-3 text-[14px] leading-[21px] tracking-[-0.2px] text-text-muted opacity-60"
                        title="Select an organization first"
                      >
                        <Icon variant={item.icon} />
                        <span className={cn(!isSidebarOpen && "md:hidden")}>{item.label}</span>
                      </span>
                    );
                  }

                  return (
                    <Link
                      key={item.label}
                      aria-label={item.label}
                      className={cn(
                        "justify-start",
                        !isSidebarOpen && "md:justify-center",
                        item.isExternal && "pr-2",
                      )}
                      href={itemHref}
                      isActive={isItemActive(item)}
                      isATag={item.isExternal}
                      isExternal={item.isExternal}
                      variant="nav"
                    >
                      <Icon variant={item.icon} />
                      <span className={cn(!isSidebarOpen && "md:hidden")}>{item.label}</span>
                      {item.isExternal ? (
                        <Icon
                          className={cn("ml-auto", !isSidebarOpen && "md:hidden")}
                          size={14}
                          variant="ArrowSquareOutIcon"
                        />
                      ) : null}
                    </Link>
                  );
                })}
              </nav>
              <hr className="border-border-subtle" />
            </div>
          ))}
        </div>
      </div>
    </aside>
  );
};