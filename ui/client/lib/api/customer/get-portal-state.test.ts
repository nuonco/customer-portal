import { beforeEach, expect, mock, test } from "bun:test";
import { getCustomerPortalState } from "./get-portal-state";

beforeEach(() => {
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      location: new URL("http://localhost:8080/"),
    },
  });

  import.meta.env.VITE_CUSTOMER_SUBDOMAIN = "test-org";

  const fetchMock = mock(async () => new Response("forbidden", { status: 403 }));
  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });
});

test("getCustomerPortalState requests portal state JSON", async () => {
  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();

    expect(url).toBe("/portal-api/state?subdomain=test-org");
    expect(init?.credentials).toBe("include");
    expect((init?.headers as Record<string, string>).Accept).toBe("application/json");

    return new Response(
      JSON.stringify({
        user: { id: "usr_1", email: "customer@example.com", name: "Customer", role: "customer" },
        org: { id: "org_1", name: "Acme", subdomain: "acme", portal_domain: "acme.localhost:8080", admin_url: "http://localhost:8080/admin/orgs/org_1" },
        theme: { header_title: "Acme", logo_light: "", logo_dark: "", primary: "#123456", primary_dark: "#0f0f0f", theme_mode: "auto" },
        active_account: null,
        other_accounts: [],
        installs: [],
        apps: [],
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  });

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  const result = await getCustomerPortalState();
  expect(result.org.name).toBe("Acme");
  expect(result.theme.primary).toBe("#123456");
});

test("getCustomerPortalState throws on non-ok response", async () => {
  await expect(getCustomerPortalState()).rejects.toBeTruthy();
});

test("getCustomerPortalState normalizes nullable list fields", async () => {
  const fetchMock = mock(async () =>
    new Response(
      JSON.stringify({
        user: { id: "usr_1", email: "customer@example.com", name: "Customer", role: "customer" },
        org: { id: "org_1", name: "Acme", subdomain: "acme", portal_domain: "acme.localhost:8080", admin_url: "http://localhost:8080/admin/orgs/org_1" },
        theme: { header_title: "Acme", logo_light: "", logo_dark: "", primary: "#123456", primary_dark: "#0f0f0f", theme_mode: "auto" },
        active_account: null,
        other_accounts: null,
        installs: null,
        apps: null,
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ),
  );

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  const result = await getCustomerPortalState();
  expect(result.other_accounts).toEqual([]);
  expect(result.installs).toEqual([]);
  expect(result.apps).toEqual([]);
});