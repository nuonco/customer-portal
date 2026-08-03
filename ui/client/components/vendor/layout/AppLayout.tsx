import { ReactNode, useEffect, useMemo, useState } from "react";
import { useLocation } from "react-router";
import { Link } from "@/components/common/Link";
import { Message } from "@/components/common/Message";
import { getVendorOrgTokenStatus } from "@/lib/api/vendor/get-org-token-status";
import { getOrgSession, getSidebarOpen } from "@/lib/cookies";
import { SidebarProvider } from "@/providers/sidebar-provider";
import { buildVendorOrgUrl } from "@/utils/vendor-url-utils";
import { VendorAppSidebar } from "@/components/vendor/layout/Sidebar";
import { VendorAppTopbar } from "@/components/vendor/layout/Topbar";

interface IVendorAppLayoutProps {
  children: ReactNode;
}

function getOrgIdFromPath(pathname: string): string | null {
  const parts = pathname.split("/").filter(Boolean);

  if (parts[0] !== "admin" || parts[1] !== "orgs") {
    return null;
  }

  return parts[2] || null;
}

function getInitialSidebarState() {
  if (typeof document === "undefined") {
    return true;
  }

  const hasCookie = document.cookie
    .split(";")
    .map((c) => c.trim())
    .some((c) => c.startsWith("sidebar_open="));

  return hasCookie ? getSidebarOpen() : true;
}

const VendorAppLayoutFrame = ({
  children,
  orgConnectionUrl,
}: IVendorAppLayoutProps & { orgConnectionUrl: string | null }) => {
  return (
    <div className="flex h-screen bg-app-bg text-text-primary">
      <VendorAppSidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <VendorAppTopbar />
        {orgConnectionUrl ? (
          <div className="px-4 pt-4 md:px-6">
            <Message theme="warning" title="Expired Token">
              <div className="mt-1">
                Generate a new token (
                <code className="text-xs font-medium">
                  nuon orgs api-token -j
                </code>
                ) and update your{" "}
                <Link href={orgConnectionUrl}>Org Connection</Link> settings to
                restore access to Nuon Control Plane APIs.
              </div>
            </Message>
          </div>
        ) : null}
        <main className="min-h-0 flex-1 p-4 md:p-6 overflow-auto">
          {children}
        </main>
      </div>
    </div>
  );
};

export const VendorAppLayout = ({ children }: IVendorAppLayoutProps) => {
  const location = useLocation();
  const initIsSidebarOpen = useMemo(() => getInitialSidebarState(), []);
  const [expiredTokenOrgId, setExpiredTokenOrgId] = useState<string | null>(
    null,
  );

  useEffect(() => {
    let isMounted = true;

    const orgId = getOrgIdFromPath(location.pathname) ?? getOrgSession();

    if (!orgId) {
      setExpiredTokenOrgId(null);
      return () => {
        isMounted = false;
      };
    }

    const checkTokenStatus = async () => {
      try {
        const status = await getVendorOrgTokenStatus(orgId);
        if (!isMounted) {
          return;
        }

        const isExpiredTitle =
          typeof status.error_title === "string" &&
          status.error_title.toLowerCase() === "token is expired";
        setExpiredTokenOrgId(!status.is_valid && isExpiredTitle ? orgId : null);
      } catch {
        if (isMounted) {
          setExpiredTokenOrgId(null);
        }
      }
    };

    void checkTokenStatus();

    return () => {
      isMounted = false;
    };
  }, [location.pathname]);

  const orgConnectionUrl = expiredTokenOrgId
    ? buildVendorOrgUrl(expiredTokenOrgId, "connection")
    : null;

  return (
    <SidebarProvider initIsSidebarOpen={initIsSidebarOpen}>
      <VendorAppLayoutFrame orgConnectionUrl={orgConnectionUrl}>
        {children}
      </VendorAppLayoutFrame>
    </SidebarProvider>
  );
};
