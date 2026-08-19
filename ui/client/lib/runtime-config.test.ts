import { afterEach, beforeEach, describe, expect, test } from "bun:test";

import {
  getRuntimeConfig,
  inferBaseDomainFromHost,
  inferCustomerSubdomain,
  withSubdomain,
} from "@/lib/runtime-config";

const originalHost = window.location.host;

// import.meta.env is shared across every test file in the process, and other
// files set these at import time (before any beforeEach runs). Snapshot and
// restore them so this file cannot leak blank values into the rest of the suite.
const originalBaseDomain = import.meta.env.VITE_SUBDOMAIN_BASE_DOMAIN;
const originalSubdomain = import.meta.env.VITE_CUSTOMER_SUBDOMAIN;
const originalApiUrl = import.meta.env.VITE_NUON_API_URL;

function setHost(host: string) {
  Object.defineProperty(window, "location", {
    value: { ...window.location, host },
    writable: true,
    configurable: true,
  });
}

beforeEach(() => {
  delete window.__PORTAL_CONFIG__;
  // These are read only as a fallback; clear them so injection is tested alone.
  import.meta.env.VITE_SUBDOMAIN_BASE_DOMAIN = "";
  import.meta.env.VITE_CUSTOMER_SUBDOMAIN = "";
  import.meta.env.VITE_NUON_API_URL = "";
});

afterEach(() => {
  setHost(originalHost);
  delete window.__PORTAL_CONFIG__;
  import.meta.env.VITE_SUBDOMAIN_BASE_DOMAIN = originalBaseDomain;
  import.meta.env.VITE_CUSTOMER_SUBDOMAIN = originalSubdomain;
  import.meta.env.VITE_NUON_API_URL = originalApiUrl;
});

describe("getRuntimeConfig", () => {
  test("prefers server-injected config over build-time env", () => {
    import.meta.env.VITE_SUBDOMAIN_BASE_DOMAIN = "from-env.example.com";
    window.__PORTAL_CONFIG__ = { subdomainBaseDomain: "injected.example.com" };

    expect(getRuntimeConfig().subdomainBaseDomain).toBe("injected.example.com");
  });

  test("falls back to build-time env when nothing is injected", () => {
    import.meta.env.VITE_SUBDOMAIN_BASE_DOMAIN = "from-env.example.com";

    expect(getRuntimeConfig().subdomainBaseDomain).toBe("from-env.example.com");
  });

  test("exposes the server's Nuon API URL", () => {
    window.__PORTAL_CONFIG__ = { nuonApiUrl: "http://localhost:8081" };

    expect(getRuntimeConfig().nuonApiUrl).toBe("http://localhost:8081");
  });

  test("nuonApiUrl is empty when the server injects nothing", () => {
    expect(getRuntimeConfig().nuonApiUrl).toBe("");
  });

  test("is not memoized, so a later injection is observed", () => {
    expect(getRuntimeConfig().subdomainBaseDomain).toBe("");

    window.__PORTAL_CONFIG__ = { subdomainBaseDomain: "late.example.com" };

    expect(getRuntimeConfig().subdomainBaseDomain).toBe("late.example.com");
  });
});

describe("inferBaseDomainFromHost", () => {
  test("uses the configured base domain when present", () => {
    window.__PORTAL_CONFIG__ = { subdomainBaseDomain: "installs.example.com" };

    expect(inferBaseDomainFromHost("acme.installs.example.com")).toBe(
      "installs.example.com",
    );
  });

  test("strips the first label when unconfigured", () => {
    expect(inferBaseDomainFromHost("acme.installs.example.com")).toBe(
      "installs.example.com",
    );
  });

  test("preserves the port when stripping", () => {
    expect(inferBaseDomainFromHost("acme.localhost:8080")).toBe("localhost:8080");
  });

  test("returns the host unchanged when there is no label to strip", () => {
    expect(inferBaseDomainFromHost("localhost:8080")).toBe("localhost:8080");
  });
});

describe("inferCustomerSubdomain", () => {
  test("extracts the subdomain against the injected base domain", () => {
    window.__PORTAL_CONFIG__ = { subdomainBaseDomain: "installs.example.com" };
    setHost("acme.installs.example.com");

    expect(inferCustomerSubdomain()).toBe("acme");
  });

  test("returns empty on the base domain itself", () => {
    window.__PORTAL_CONFIG__ = { subdomainBaseDomain: "installs.example.com" };
    setHost("installs.example.com");

    expect(inferCustomerSubdomain()).toBe("");
  });

  test("uses the dev fallback when on a host with no subdomain", () => {
    window.__PORTAL_CONFIG__ = {
      subdomainBaseDomain: "localhost:8080",
      customerSubdomain: "dev-org",
    };
    setHost("localhost:8080");

    expect(inferCustomerSubdomain()).toBe("dev-org");
  });

  test("prefers the real subdomain over the fallback", () => {
    window.__PORTAL_CONFIG__ = {
      subdomainBaseDomain: "installs.example.com",
      customerSubdomain: "dev-org",
    };
    setHost("acme.installs.example.com");

    expect(inferCustomerSubdomain()).toBe("acme");
  });
});

describe("withSubdomain", () => {
  test("appends the subdomain as a query param", () => {
    window.__PORTAL_CONFIG__ = { subdomainBaseDomain: "installs.example.com" };
    setHost("acme.installs.example.com");

    expect(withSubdomain("/portal-api/state")).toBe(
      "/portal-api/state?subdomain=acme",
    );
  });

  test("omits the param entirely when there is no subdomain", () => {
    window.__PORTAL_CONFIG__ = { subdomainBaseDomain: "installs.example.com" };
    setHost("installs.example.com");

    expect(withSubdomain("/portal-api/state")).toBe("/portal-api/state");
  });
});
