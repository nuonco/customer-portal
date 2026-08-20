import { useCallback, useContext, useEffect, useMemo, useState } from "react";
import { type TIconVariant } from "@/components/common/Icon";
import { CustomerNavigationContext } from "@/providers/customer-navigation-provider";
import {
  type TCustomerPortalApp,
  type TCustomerPortalInstall,
  type TCustomerPortalState,
} from "@/lib/api/customer/get-portal-state";

export type TInstallTab = {
  icon: TIconVariant;
  key: string;
  label: string;
};

export const INSTALL_TABS: TInstallTab[] = [
  { key: "overview", label: "Overview", icon: "HouseSimpleIcon" },
  { key: "stack", label: "Stack", icon: "StackIcon" },
  { key: "sandbox", label: "Sandbox", icon: "PackageIcon" },
  { key: "components", label: "Components", icon: "SquaresFourIcon" },
  { key: "roles", label: "Roles", icon: "ShieldCheckIcon" },
  { key: "policies", label: "Policies", icon: "ShieldCheckIcon" },
  { key: "audit", label: "Audit Log", icon: "ClockCounterClockwiseIcon" },
  { key: "readme", label: "README", icon: "BookOpenIcon" },
];

const INSTALL_TAB_KEYS = new Set(INSTALL_TABS.map((item) => item.key));

function tabTitle(tab: string | null): string {
  return INSTALL_TABS.find((item) => item.key === tab)?.label ?? "Overview";
}

function getActiveInstallTab(
  pathname: string,
  installId: string | null,
): string | null {
  if (!installId) {
    return null;
  }

  const installBasePath = `/installs/${installId}`;
  if (pathname === installBasePath) {
    return "overview";
  }

  if (!pathname.startsWith(`${installBasePath}/`)) {
    return null;
  }

  const nextSegment = pathname.slice(installBasePath.length + 1).split("/")[0];
  return INSTALL_TAB_KEYS.has(nextSegment) ? nextSegment : "overview";
}

export type TCustomerPortalNavigation = {
  currentInstall: TCustomerPortalInstall | null;
  currentApp: TCustomerPortalApp | null;
  activeInstallTab: string | null;
  pageTitle: string;
  pageSubtitle: string;
  isAllInstallsActive: boolean;
  isAppCatalogActive: boolean;
  isSettingsActive: boolean;
  isSidebarMinimized: boolean;
  isMobileSidebarOpen: boolean;
  isSidebarCollapsed: boolean;
  toggleSidebarMinimized: () => void;
  openMobileSidebar: () => void;
  closeMobileSidebar: () => void;
};

type TUseNavigationInput = {
  pathname: string;
  search: string;
  installId: string | null;
  appId: string | null;
  portalState: TCustomerPortalState | null;
};

const SIDEBAR_COOKIE = "sidebar_minimized";

function readSidebarMinimized(): boolean {
  if (typeof document === "undefined") {
    return false;
  }

  return document.cookie
    .split(";")
    .map((cookie) => cookie.trim())
    .some((cookie) => cookie === `${SIDEBAR_COOKIE}=true`);
}

function writeSidebarMinimized(nextValue: boolean) {
  document.cookie = `${SIDEBAR_COOKIE}=${String(nextValue)};path=/;max-age=31536000;SameSite=Lax`;
}

export function useCustomerNavigation(): TCustomerPortalNavigation {
  const context = useContext(CustomerNavigationContext);
  if (!context) {
    throw new Error(
      "useCustomerNavigation must be used inside CustomerNavigationProvider",
    );
  }

  return context;
}

export function useNavigation({
  pathname,
  search,
  installId,
  appId,
  portalState,
}: TUseNavigationInput): TCustomerPortalNavigation {
  const [isSidebarMinimized, setIsSidebarMinimized] = useState(() =>
    readSidebarMinimized(),
  );
  const [isMobileSidebarOpen, setIsMobileSidebarOpen] = useState(false);

  useEffect(() => {
    setIsMobileSidebarOpen(false);
  }, [pathname, search]);

  const toggleSidebarMinimized = useCallback(() => {
    setIsSidebarMinimized((current) => {
      const nextValue = !current;
      writeSidebarMinimized(nextValue);
      return nextValue;
    });
  }, []);

  const openMobileSidebar = useCallback(() => {
    setIsMobileSidebarOpen(true);
  }, []);

  const closeMobileSidebar = useCallback(() => {
    setIsMobileSidebarOpen(false);
  }, []);

  const navigation = useMemo(() => {
    if (!portalState) {
      return {
        currentInstall: null,
        currentApp: null,
        activeInstallTab: null,
        pageTitle: pathname.startsWith("/account")
          ? "My Group"
          : pathname.startsWith("/apps")
            ? "Apps"
            : "Installs",
        pageSubtitle: "",
        isAllInstallsActive: pathname === "/installs",
        isAppCatalogActive: pathname === "/apps" || pathname.startsWith("/apps/"),
        isSettingsActive: pathname.startsWith("/account"),
      };
    }

    const currentInstall = installId
      ? (portalState.installs.find((install) => install.id === installId) ?? null)
      : null;
    const currentApp = appId
      ? (portalState.apps.find((app) => app.app_id === appId) ?? null)
      : null;
    const activeInstallTab = getActiveInstallTab(pathname, currentInstall?.id ?? null);
    const isInstallWizardRoute =
      pathname.startsWith("/apps/") && pathname.endsWith("/install");

    const pageSubtitle =
      isInstallWizardRoute && currentApp
        ? `App ID: ${currentApp.app_id}`
        : (portalState.active_account?.name ?? portalState.org.name);
    const pageTitle = currentInstall
      ? `${currentInstall.name} · ${tabTitle(activeInstallTab)}`
      : currentApp
        ? currentApp.display_name
        : pathname.startsWith("/account")
          ? "My Group"
          : pathname.startsWith("/apps")
            ? "Apps"
            : "Installs";

    return {
      currentInstall,
      currentApp,
      activeInstallTab,
      pageTitle,
      pageSubtitle,
      isAllInstallsActive: pathname === "/installs",
      isAppCatalogActive: pathname === "/apps" || pathname.startsWith("/apps/"),
      isSettingsActive: pathname.startsWith("/account"),
    };
  }, [appId, installId, pathname, portalState]);

  return {
    ...navigation,
    isSidebarMinimized,
    isMobileSidebarOpen,
    isSidebarCollapsed: isSidebarMinimized && !isMobileSidebarOpen,
    toggleSidebarMinimized,
    openMobileSidebar,
    closeMobileSidebar,
  };
}
