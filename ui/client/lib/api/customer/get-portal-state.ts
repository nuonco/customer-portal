import { extractSubdomain } from "@/utils/subdomain-utils";

export type TCustomerPortalTheme = {
  header_title: string;
  logo_light: string;
  logo_dark: string;
  primary: string;
  primary_dark: string;
  theme_mode: "auto" | "light" | "dark";
};

export type TCustomerPortalUser = {
  id: string;
  email: string;
  name: string;
  role: string;
};

export type TCustomerPortalOrg = {
  id: string;
  name: string;
  subdomain: string;
  portal_domain: string;
  admin_url: string;
};

export type TCustomerPortalAccount = {
  id: string;
  name: string;
};

export type TCustomerPortalInstall = {
  id: string;
  name: string;
  app_id: string;
  app_name: string;
  status: string;
  visibility: string;
  created_at: string;
  app_logo_light: string;
  app_logo_dark: string;
  legacy_base_path: string;
};

export type TCustomerPortalApp = {
  id: string;
  app_id: string;
  display_name: string;
  status: string;
  summary: string;
  platform: string;
  logo_light: string;
  logo_dark: string;
  legacy_base_path: string;
};

export type TCustomerPortalState = {
  user: TCustomerPortalUser;
  org: TCustomerPortalOrg;
  theme: TCustomerPortalTheme;
  active_account: TCustomerPortalAccount | null;
  other_accounts: TCustomerPortalAccount[];
  installs: TCustomerPortalInstall[];
  apps: TCustomerPortalApp[];
};

function inferCustomerSubdomain(): string {
  if (typeof window === "undefined") {
    return "";
  }

  const configuredBaseDomain =
    (import.meta.env.VITE_SUBDOMAIN_BASE_DOMAIN as string | undefined) ?? "";
  const fallbackSubdomain =
    (import.meta.env.VITE_CUSTOMER_SUBDOMAIN as string | undefined) ?? "";

  const inferredBaseDomain = (() => {
    if (configuredBaseDomain) {
      return configuredBaseDomain;
    }

    const [hostname, port] = window.location.host.split(":");
    if (!hostname) {
      return window.location.host;
    }

    const labels = hostname.split(".");
    if (labels.length <= 1) {
      return window.location.host;
    }

    const host = labels.slice(1).join(".");
    return port ? `${host}:${port}` : host;
  })();

  return extractSubdomain(window.location.host, inferredBaseDomain) || fallbackSubdomain;
}

export async function getCustomerPortalState(): Promise<TCustomerPortalState> {
  const query = new URLSearchParams();
  const subdomain = inferCustomerSubdomain();
  if (subdomain) {
    query.set("subdomain", subdomain);
  }

  const response = await fetch(`/bff/portal-api/state${query.toString() ? `?${query.toString()}` : ""}`, {
    credentials: "include",
    headers: { Accept: "application/json" },
  });

  if (!response.ok) {
    throw new Error(`Customer portal state request failed with status ${response.status}`);
  }

  const state = (await response.json()) as TCustomerPortalState;

  return {
    ...state,
    other_accounts: state.other_accounts ?? [],
    installs: state.installs ?? [],
    apps: state.apps ?? [],
  };
}