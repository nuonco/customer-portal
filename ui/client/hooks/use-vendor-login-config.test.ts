import { beforeEach, expect, test } from "bun:test";
import {
  buildVendorLoginStateRedirect,
  normalizeVendorRedirectPath,
} from "./use-vendor-login-config";

beforeEach(() => {
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      location: {
        hostname: "localhost",
        port: "51273",
      },
    },
  });
});

test("normalizeVendorRedirectPath defaults to /admin/orgs when redirect is missing", () => {
  expect(normalizeVendorRedirectPath(null)).toBe("/admin/orgs");
  expect(normalizeVendorRedirectPath("")).toBe("/admin/orgs");
});

test("normalizeVendorRedirectPath allows safe relative admin route", () => {
  expect(normalizeVendorRedirectPath("/admin/orgs/org-1/installs")).toBe(
    "/admin/orgs/org-1/installs",
  );
});

test("normalizeVendorRedirectPath rejects non-relative redirect values", () => {
  expect(normalizeVendorRedirectPath("admin/orgs")).toBe("/admin/orgs");
  expect(normalizeVendorRedirectPath("https://evil.example/path")).toBe(
    "/admin/orgs",
  );
  expect(normalizeVendorRedirectPath("//evil.example/path")).toBe(
    "/admin/orgs",
  );
});

test("buildVendorLoginStateRedirect wraps localhost redirects for ui_port bridge", () => {
  expect(buildVendorLoginStateRedirect("/admin/orgs/org-1/installs")).toBe(
    "/auth-api/post-login?ui_port=51273&next=%2Fadmin%2Forgs%2Forg-1%2Finstalls",
  );
});

test("buildVendorLoginStateRedirect returns direct path on backend port", () => {
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      location: {
        hostname: "localhost",
        port: "8080",
      },
    },
  });

  expect(buildVendorLoginStateRedirect("/admin/orgs")).toBe("/admin/orgs");
});
