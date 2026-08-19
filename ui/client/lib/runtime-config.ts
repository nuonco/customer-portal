import { extractSubdomain } from "@/utils/subdomain-utils";

/**
 * Environment-specific values the client needs at runtime.
 *
 * These are injected by the Go server into index.html as
 * `window.__PORTAL_CONFIG__` (see internal/spa/serve.go) rather than baked in at
 * build time. The subdomain base domain differs between stage and prod, so
 * reading it from `import.meta.env` would require building a separate bundle —
 * and therefore a separate container image — per environment.
 *
 * The `import.meta.env` fallback is retained for contexts that serve the bundle
 * without the Go server: Ladle and unit tests.
 */
export type TRuntimeConfig = {
  /**
   * Domain that org subdomains hang off of, e.g. "installs.nuon.co". Subtracted
   * from window.location.host to determine which org's portal is rendering.
   */
  subdomainBaseDomain: string;
  /** Development-only fallback for hosts with no subdomain. Empty in prod. */
  customerSubdomain: string;
  /**
   * The upstream Nuon API the server talks to. Forms that let you override it
   * should show this as the placeholder, since it is what a blank field
   * actually resolves to server-side.
   */
  nuonApiUrl: string;
  version?: string;
  gitRef?: string;
};

declare global {
  interface Window {
    __PORTAL_CONFIG__?: Partial<TRuntimeConfig>;
  }
}

function readEnv(key: string): string {
  // import.meta.env is absent under some test runners.
  const env = (import.meta as { env?: Record<string, unknown> }).env;
  const value = env?.[key];
  return typeof value === "string" ? value : "";
}

/**
 * Deliberately not memoized. Reading a global is cheap, and caching would leak
 * state between test files (bun shares one process across them) and between
 * Ladle stories that override the config.
 */
export function getRuntimeConfig(): TRuntimeConfig {
  const injected =
    typeof window === "undefined" ? undefined : window.__PORTAL_CONFIG__;

  return {
    subdomainBaseDomain:
      injected?.subdomainBaseDomain ?? readEnv("VITE_SUBDOMAIN_BASE_DOMAIN"),
    customerSubdomain:
      injected?.customerSubdomain ?? readEnv("VITE_CUSTOMER_SUBDOMAIN"),
    nuonApiUrl: injected?.nuonApiUrl ?? readEnv("VITE_NUON_API_URL"),
    version: injected?.version,
    gitRef: injected?.gitRef,
  };
}

/**
 * Resolves the base domain for a host. Prefers the configured value; otherwise
 * assumes the first label of the host is the subdomain and strips it.
 */
export function inferBaseDomainFromHost(host: string): string {
  const configured = getRuntimeConfig().subdomainBaseDomain;
  if (configured) {
    return configured;
  }

  const [hostname, port] = host.split(":");
  if (!hostname) {
    return host;
  }

  const labels = hostname.split(".");
  if (labels.length <= 1) {
    return host;
  }

  const base = labels.slice(1).join(".");
  return port ? `${base}:${port}` : base;
}

/**
 * Returns the org subdomain the app is currently being viewed under, or "" when
 * on the base domain.
 */
export function inferCustomerSubdomain(): string {
  if (typeof window === "undefined") {
    return "";
  }

  const { host } = window.location;
  return (
    extractSubdomain(host, inferBaseDomainFromHost(host)) ||
    getRuntimeConfig().customerSubdomain
  );
}

/**
 * Appends the current org subdomain as a query param. The server needs it
 * because requests may arrive on the base domain during auth transitions.
 */
export function withSubdomain(pathname: string): string {
  const query = new URLSearchParams();
  const subdomain = inferCustomerSubdomain();
  if (subdomain) {
    query.set("subdomain", subdomain);
  }
  return `${pathname}${query.toString() ? `?${query.toString()}` : ""}`;
}
