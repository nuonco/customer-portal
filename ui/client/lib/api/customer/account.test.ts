import { beforeEach, expect, mock, test } from "bun:test";
import { createCustomerAccount, switchCustomerAccount } from "./account";

beforeEach(() => {
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      location: new URL("http://localhost:8080/"),
    },
  });

  import.meta.env.VITE_CUSTOMER_SUBDOMAIN = "acme";
});

test("createCustomerAccount posts to portal API", async () => {
  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();
    expect(url).toBe("/portal-api/accounts?subdomain=acme");
    expect(init?.method).toBe("POST");
    expect((init?.headers as Record<string, string>).Accept).toBe("application/json");
    expect(init?.body).toBe(JSON.stringify({ name: "Platform Team" }));
    return new Response(JSON.stringify({ account: { id: "acct_1", name: "Platform Team" } }), {
      status: 201,
      headers: { "Content-Type": "application/json" },
    });
  });

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  await createCustomerAccount("Platform Team");
});

test("switchCustomerAccount posts to portal API", async () => {
  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();
    expect(url).toBe("/portal-api/accounts/switch?subdomain=acme");
    expect(init?.method).toBe("POST");
    expect(init?.body).toBe(JSON.stringify({ account_id: "acct_2" }));
    return new Response(JSON.stringify({ ok: true }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  });

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  await switchCustomerAccount("acct_2");
});

test("createCustomerAccount returns API error message when available", async () => {
  const fetchMock = mock(async () =>
    new Response(JSON.stringify({ error: "Group name is required" }), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    }),
  );

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  await expect(createCustomerAccount("   ")).rejects.toThrow("Group name is required");
});
