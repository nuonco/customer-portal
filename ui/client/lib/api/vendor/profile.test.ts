import { afterEach, describe, expect, it, vi } from "bun:test";
import { updateVendorProfile } from "./profile";

describe("updateVendorProfile", () => {
  const originalFetch = globalThis.fetch;

  afterEach(() => {
    globalThis.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it("calls the expected endpoint with trimmed display name payload", async () => {
    globalThis.fetch = vi.fn(async () => ({
      ok: true,
      json: async () => ({}),
    })) as unknown as typeof fetch;

    await expect(updateVendorProfile("Ada Lovelace")).resolves.toBeUndefined();

    expect(globalThis.fetch).toHaveBeenCalledWith("/admin/profile", {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      body: JSON.stringify({ name: "Ada Lovelace" }),
    });
  });

  it("throws backend error message when request fails", async () => {
    globalThis.fetch = vi.fn(async () => ({
      ok: false,
      json: async () => ({ error: "Display name is required" }),
    })) as unknown as typeof fetch;

    await expect(updateVendorProfile("")).rejects.toThrow(
      "Display name is required",
    );
  });
});
