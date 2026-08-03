import {
  createBrowserRouter,
  Navigate,
  Outlet,
  useLocation,
} from "react-router";
import { CustomerAuthLayout } from "@/components/customer/layout/AuthLayout";
import { CustomerPortalLayout } from "@/components/customer/layout/PortalLayout";
import { VendorAuthLayout } from "@/components/vendor/layout/AuthLayout";

import { CustomerAccountView } from "@/views/customer/Account";
import { CustomerAppDetailView, CustomerAppsView } from "@/views/customer/Apps";
import { CustomerInstallWizardView } from "@/views/customer/InstallWizard";
import { CustomerInstallsView } from "@/views/customer/Installs";
import { CustomerSelectedInstallProvider } from "@/views/customer/install/SelectedInstallProvider";
import { CustomerInstallOverviewView } from "@/views/customer/install/Overview";
import { CustomerInstallStackView } from "@/views/customer/install/Stack";
import { CustomerInstallSandboxView } from "@/views/customer/install/Sandbox";
import { CustomerInstallComponentsView } from "@/views/customer/install/Components";
import { CustomerInstallRolesView } from "@/views/customer/install/Roles";
import { CustomerInstallPoliciesView } from "@/views/customer/install/Policies";
import { CustomerInstallAuditView } from "@/views/customer/install/Audit";
import { CustomerInstallReadmeView } from "@/views/customer/install/Readme";
import { OrgsView } from "@/views/vendor/Orgs";
import { VendorLoginView } from "@/views/vendor/Login";
import { RouteError } from "@/views/RouteError";
import { SurfacesProvider } from "@/providers/surfaces-provider";
import { VendorAuthProvider } from "@/providers/vendor-auth-provider";
import { AccountsView } from "@/views/vendor/accounts/Accounts";
import { AccountDetailView } from "@/views/vendor/accounts/AccountDetail";
import { AccountMembersView } from "@/views/vendor/accounts/AccountMembers";
import { AccountInstallsView } from "@/views/vendor/accounts/AccountInstalls";
import { InstallLinksView } from "@/views/vendor/install-links/InstallLinks";
import { InstallLinkDetailView } from "@/views/vendor/install-links/InstallLinkDetail";
import { AppsView } from "@/views/vendor/apps/Apps";
import { AppInputsView } from "@/views/vendor/apps/AppInputs";
import { AppLogoView } from "@/views/vendor/apps/AppLogo";
import { AppOverviewView } from "@/views/vendor/apps/AppOverview";
import { InstallsView } from "@/views/vendor/installs/Installs";
import { CustomersView } from "@/views/vendor/customers/Customers";
import { TeamView } from "@/views/vendor/team/Team";
import { BrandingView } from "@/views/vendor/portal/Branding";
import { CustomThemeView } from "@/views/vendor/portal/CustomTheme";
import { PortalLoginView } from "@/views/vendor/portal/PortalLogin";
import { PortalDnsView } from "@/views/vendor/portal/PortalDns";
import { OrgConnectionView } from "@/views/vendor/connection/OrgConnection";
import { DebugView } from "@/views/vendor/debug/Debug";
import { SuperuserView } from "@/views/vendor/superuser/Superuser";
import { CustomerAuthProvider } from "@/providers/customer-auth-provider";
import { extractSubdomain } from "@/utils/subdomain-utils";

const VendorAuthTree = () => (
  <VendorAuthProvider>
    <SurfacesProvider>
      <Outlet />
    </SurfacesProvider>
  </VendorAuthProvider>
)

const CustomerAuthTree = () => (
  <CustomerAuthProvider>
    <SurfacesProvider>
      <Outlet />
    </SurfacesProvider>
  </CustomerAuthProvider>
)

const CanonicalPathGuard = () => {
  const location = useLocation();
  const { hash, pathname, search } = location;

  if (pathname.length > 1 && pathname.endsWith("/")) {
    const canonicalPath = pathname.replace(/\/+$/, "");
    return <Navigate to={`${canonicalPath}${search}${hash}`} replace />;
  }

  return <Outlet />;
};

const configuredBaseDomain =
  (import.meta.env.VITE_SUBDOMAIN_BASE_DOMAIN as string | undefined) ?? "";

function inferBaseDomainFromHost(host: string): string {
  if (configuredBaseDomain) {
    return configuredBaseDomain;
  }

  const [hostname, port] = host.split(":");
  if (!hostname) {
    return host;
  }

  const labels = hostname.split(".");
  if (labels.length <= 1) {
    return host;
  }

  const inferredHost = labels.slice(1).join(".");
  return port ? `${inferredHost}:${port}` : inferredHost;
}

const CustomerSubdomainGuard = () => {
  const baseDomain = inferBaseDomainFromHost(window.location.host);
  const subdomain = extractSubdomain(window.location.host, baseDomain);

  if (!subdomain) {
    return <Navigate to="/admin/orgs" replace />;
  }

  return <Outlet />;
};

export const router = createBrowserRouter([
  {
    element: <CanonicalPathGuard />,
    children: [
      {
        path: "/",
        element: <CustomerSubdomainGuard />,
        children: [
          {
            element: <CustomerAuthTree />,
            children: [
              {
                element: <CustomerAuthLayout />,
                children: [
                  {
                    element: <CustomerPortalLayout />,
                    children: [
                      {
                        index: true,
                        element: <Navigate to="installs" replace />,
                      },
                      {
                        path: "apps",
                        children: [
                          {
                            index: true,
                            element: <CustomerAppsView />,
                          },
                          {
                            path: ":appId/install",
                            element: <CustomerInstallWizardView />,
                          },
                          {
                            path: ":appId",
                            element: <CustomerAppDetailView />,
                          },
                        ],
                      },
                      {
                        path: "installs",
                        children: [
                          {
                            index: true,
                            element: <CustomerInstallsView />,
                          },
                          {
                            path: ":installId",
                            element: <CustomerSelectedInstallProvider />,
                            children: [
                              {
                                index: true,
                                element: <Navigate to="overview" replace />,
                              },
                              {
                                path: "overview",
                                element: <CustomerInstallOverviewView />,
                              },
                              {
                                path: "stack",
                                element: <CustomerInstallStackView />,
                              },
                              {
                                path: "sandbox",
                                element: <CustomerInstallSandboxView />,
                              },
                              {
                                path: "components",
                                element: <CustomerInstallComponentsView />,
                              },
                              {
                                path: "roles",
                                element: <CustomerInstallRolesView />,
                              },
                              {
                                path: "policies",
                                element: <CustomerInstallPoliciesView />,
                              },
                              {
                                path: "audit",
                                element: <CustomerInstallAuditView />,
                              },
                              {
                                path: "readme",
                                element: <CustomerInstallReadmeView />,
                              },
                              {
                                path: "*",
                                element: <Navigate to="overview" replace />,
                              },
                            ],
                          },
                        ],
                      },
                      {
                        path: "account",
                        element: <CustomerAccountView />,
                      },
                      {
                        path: "*",
                        element: <Navigate to="/installs" replace />,
                      },
                    ],
                  },
                ],
              },
            ],
          },
        ],
      },
      {
        path: "/customer",
        element: <Navigate to="/" replace />,
      },
      {
        path: "/admin/login",
        element: <VendorLoginView />,
      },
      {
        path: "/admin",
        element: <VendorAuthTree />,
        children: [
          {
            element: <VendorAuthLayout />,
            errorElement: <RouteError />,
            children: [
              {
                index: true,
                element: <Navigate to="orgs" replace />,
              },
              {
                path: "orgs",
                children: [
                  {
                    index: true,
                    element: <OrgsView />,
                  },
                  {
                    // orgId is a 26-char NanoID with prefix "ino" (local NuonOrg.ID)
                    path: ":orgId",
                    children: [
                      {
                        index: true,
                        element: <Navigate to="accounts" replace />,
                      },
                      {
                        path: "accounts",
                        children: [
                          { index: true, element: <AccountsView /> },
                          {
                            path: ":accountId",
                            children: [
                              { index: true, element: <AccountDetailView /> },
                              { path: "members", element: <AccountMembersView /> },
                              { path: "installs", element: <AccountInstallsView /> },
                            ],
                          },
                        ],
                      },
                      {
                        path: "install-links",
                        children: [
                          { index: true, element: <InstallLinksView /> },
                          { path: ":linkId", element: <InstallLinkDetailView /> },
                        ],
                      },
                      {
                        path: "apps",
                        children: [
                          { index: true, element: <AppsView /> },
                          {
                            path: ":appId",
                            children: [
                              { index: true, element: <Navigate to="inputs" replace /> },
                              { path: "inputs", element: <AppInputsView /> },
                              { path: "logo", element: <AppLogoView /> },
                              { path: "overview", element: <AppOverviewView /> },
                            ],
                          },
                        ],
                      },
                      { path: "installs", element: <InstallsView /> },
                      { path: "customers", element: <CustomersView /> },
                      {
                        path: "team",
                        children: [
                          { index: true, element: <Navigate to="members" replace /> },
                          { path: "members", element: <TeamView /> },
                        ],
                      },
                      {
                        path: "portal",
                        children: [
                          { index: true, element: <Navigate to="branding" replace /> },
                          { path: "branding", element: <BrandingView /> },
                          { path: "custom-theme", element: <CustomThemeView /> },
                          { path: "login", element: <PortalLoginView /> },
                          { path: "dns", element: <PortalDnsView /> },
                        ],
                      },
                      { path: "connection", element: <OrgConnectionView /> },
                    ],
                  },
                ],
              },
              {
                path: "superuser",
                children: [
                  { index: true, element: <SuperuserView /> },
                  { path: "*", element: <SuperuserView /> },
                ],
              },
              {
                path: "debug",
                children: [
                  { index: true, element: <DebugView /> },
                  { path: "*", element: <DebugView /> },
                ],
              },
              {
                path: "*",
                element: <Navigate to="/admin/orgs" replace />,
              },
            ],
          },
        ],
      },
      {
        path: "*",
        element: <Navigate to="/admin/orgs" replace />,
      },
    ],
  },
]);
