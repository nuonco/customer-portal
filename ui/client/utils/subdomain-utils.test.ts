import { describe, expect, test } from "bun:test";
import {
  extractSubdomain,
  generateSubdomainCandidates,
  isCustomerBaseDomainRedirectPath,
  normalizeSubdomain,
  shouldRedirectBaseDomainCustomerRoute,
  validateSubdomain,
} from "./subdomain-utils";

describe("subdomain-utils", () => {
  test("normalizes names with spaces and punctuation", () => {
    expect(normalizeSubdomain("Acme Cloud, Inc.")).toBe("acme-cloud-inc");
  });

  test("falls back to org when name normalizes to empty", () => {
    expect(normalizeSubdomain("***")).toBe("org");
  });

  test("rejects reserved subdomains", () => {
    expect(validateSubdomain("admin")).toBe("'admin' is a reserved subdomain");
  });

  test("accepts valid subdomains", () => {
    expect(validateSubdomain("acme-1")).toBeNull();
  });

  test("generates incrementing fallback candidates", () => {
    expect(generateSubdomainCandidates("Acme", 4)).toEqual(["acme", "acme-1", "acme-2", "acme-3"]);
  });

  test("extracts subdomain using matching base domain", () => {
    expect(extractSubdomain("test-org.localhost:8080", "localhost:8080")).toBe("test-org");
  });

  test("returns empty subdomain for non-matching host", () => {
    expect(extractSubdomain("localhost:8080", "localhost:8080")).toBe("");
  });

  test("matches backend customer base-domain redirect paths", () => {
    expect(isCustomerBaseDomainRedirectPath("/")).toBe(true);
    expect(isCustomerBaseDomainRedirectPath("/login")).toBe(true);
    expect(isCustomerBaseDomainRedirectPath("/installs/abc")).toBe(true);
    expect(isCustomerBaseDomainRedirectPath("/apps")).toBe(false);
  });

  test("redirect decision matches host+path policy", () => {
    expect(shouldRedirectBaseDomainCustomerRoute("localhost:8080", "localhost:8080", "/installs")).toBe(true);
    expect(shouldRedirectBaseDomainCustomerRoute("test.localhost:8080", "localhost:8080", "/installs")).toBe(false);
  });
});