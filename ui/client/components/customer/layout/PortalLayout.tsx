import { type CSSProperties } from "react";
import { Outlet, useLocation, useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { Message } from "@/components/common/Message";
import { Text } from "@/components/common/Text";
import { getCustomerPortalState } from "@/lib/api/customer/get-portal-state";
import {
  CustomerPortalStateProvider,
  type TCustomerPortalContext,
} from "@/providers/customer-portal-state-provider";
import { useNavigation } from "@/hooks/use-customer-navigation";
import { CustomerNavigationProvider } from "@/providers/customer-navigation-provider";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";

export const CustomerPortalLayout = () => {
  const location = useLocation();
  const params = useParams();

  const { data, error, isLoading, refetch } = useQuery({
    queryKey: ["customer", "portal", "state"],
    queryFn: getCustomerPortalState,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
  });

  const navigation = useNavigation({
    pathname: location.pathname,
    search: location.search,
    installId: params.installId ?? null,
    appId: params.appId ?? null,
    portalState: data ?? null,
  });

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center px-4">
        <Text variant="body" theme="neutral">
          Loading your customer portal...
        </Text>
      </div>
    );
  }

  if (!data) {
    return (
      <div className="mx-auto flex min-h-screen w-full max-w-3xl items-center px-4">
        <Message theme="warning">
          {error instanceof Error
            ? error.message
            : "Unable to load your customer portal."}
        </Message>
      </div>
    );
  }

  const style = {
    "--portal-primary": data.theme.primary,
    "--portal-primary-dark": data.theme.primary_dark,
  } as CSSProperties;

  const portalContext: TCustomerPortalContext = {
    portalState: data,
    currentInstall: navigation.currentInstall,
    currentApp: navigation.currentApp,
    activeInstallTab: navigation.activeInstallTab,
    refetchPortalState: async () => refetch(),
  };

  return (
    <CustomerPortalStateProvider value={portalContext}>
      <CustomerNavigationProvider navigation={navigation}>
        <div className="h-screen bg-gradient" style={style}>
          <div className="flex h-screen">
            {navigation.isMobileSidebarOpen ? (
              <button
                type="button"
                aria-label="Close sidebar"
                className="fixed inset-0 z-30 bg-black/35 md:hidden"
                onClick={navigation.closeMobileSidebar}
              />
            ) : null}

            <Sidebar portalState={data} />

            <div className="flex min-w-0 flex-1 flex-col h-screen overflow-y-auto">
              <Topbar portalState={data} />

              <main className="min-w-0 flex-1 px-4 py-5 md:px-6 md:py-6">
                <div className="mx-auto flex w-full max-w-7xl flex-col gap-4">
                  <Outlet />
                </div>
              </main>
            </div>
          </div>
        </div>
      </CustomerNavigationProvider>
    </CustomerPortalStateProvider>
  );
};
