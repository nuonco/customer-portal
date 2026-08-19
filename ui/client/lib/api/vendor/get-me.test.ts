import { beforeEach, expect, mock, test } from "bun:test";
import {
  getMe,
  VendorAuthFailureError,
  VendorAuthUnavailableError,
} from "./get-me";

beforeEach(() => {
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      location: new URL("http://localhost:8080/admin/orgs"),
    },
  });
});

test("returns user profile when vendor session is valid", async () => {
  const fetchMock = mock(async () =>
    new Response(
      JSON.stringify({
        id: "usr_1",
        email: "vendor@example.com",
        name: "Vendor User",
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ),
  );

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  await expect(getMe()).resolves.toEqual({
    id: "usr_1",
    email: "vendor@example.com",
    name: "Vendor User",
  });
});

test("throws auth failure on 401", async () => {
  const fetchMock = mock(async () => new Response("unauthorized", { status: 401 }));

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  await expect(getMe()).rejects.toBeInstanceOf(VendorAuthFailureError);
});

test("throws auth failure when request is redirected to vendor login", async () => {
  const fetchMock = mock(async () => {
    const response = new Response("<html>login</html>", {
      status: 200,
      headers: { "Content-Type": "text/html" },
    });

    return Object.assign(response, {
      redirected: true,
      url: "http://localhost:8080/admin/login",
    });
  });

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  await expect(getMe()).rejects.toBeInstanceOf(VendorAuthFailureError);
});

test("throws auth failure when login HTML is returned without redirect metadata", async () => {
  const fetchMock = mock(async () =>
    Object.assign(
      new Response("<html>login</html>", {
        status: 200,
        headers: { "Content-Type": "text/html" },
      }),
      { url: "http://localhost:8080/login" },
    ),
  );

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  await expect(getMe()).rejects.toBeInstanceOf(VendorAuthFailureError);
});

test("throws unavailable error on non-auth non-ok responses", async () => {
  const fetchMock = mock(async () => new Response("server error", { status: 500 }));

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  await expect(getMe()).rejects.toBeInstanceOf(VendorAuthUnavailableError);
});
