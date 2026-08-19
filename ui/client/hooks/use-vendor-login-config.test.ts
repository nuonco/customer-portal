import { expect, test } from "bun:test";
import { normalizeVendorRedirectPath } from "./use-vendor-login-config";

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
