import { beforeEach, expect, mock, test } from "bun:test";
import {
  forgetDeletedApp,
  getAppCatalog,
  updateAppCatalog,
} from "./get-app-catalog";

beforeEach(() => {
  const fetchMock = mock(async () => new Response("not found", { status: 404 }));
  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });
});

test("getAppCatalog requests JSON payload from catalog endpoint", async () => {
  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();

    expect(url).toBe("/admin/orgs/org-1/apps-api/catalog?page=2");
    expect(init?.credentials).toBe("include");
    expect((init?.headers as Record<string, string>).Accept).toBe("application/json");

    return new Response(
      JSON.stringify({
        apps: [],
        pagination: {
          current_page: 2,
          has_previous: true,
          has_next: false,
          previous_page: 1,
          next_page: 3,
        },
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  });

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  const result = await getAppCatalog("org-1", { page: 2 });
  expect(result.pagination.current_page).toBe(2);
});

test("updateAppCatalog sends expected payload to save endpoint", async () => {
  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();

    expect(url).toBe("/admin/orgs/org-1/apps/order");
    expect(init?.method).toBe("PUT");
    expect((init?.headers as Record<string, string>)["Content-Type"]).toBe("application/json");

    const body = JSON.parse(String(init?.body)) as {
      app_ids: string[];
      app_statuses: Record<string, string>;
    };
    expect(body.app_ids).toEqual(["app-1", "app-2"]);
    expect(body.app_statuses["app-1"]).toBe("published");

    return new Response(JSON.stringify({ message: "ok" }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  });

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  await updateAppCatalog("org-1", {
    app_ids: ["app-1", "app-2"],
    app_statuses: {
      "app-1": "published",
      "app-2": "unpublished",
    },
  });

  expect(fetchMock).toHaveBeenCalledTimes(1);
});

test("forgetDeletedApp sends delete request to forget endpoint", async () => {
  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();

    expect(url).toBe("/admin/orgs/org-1/apps/app-1/forget");
    expect(init?.method).toBe("DELETE");
    expect(init?.credentials).toBe("include");

    return new Response(JSON.stringify({ message: "ok" }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  });

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  await forgetDeletedApp("org-1", "app-1");
  expect(fetchMock).toHaveBeenCalledTimes(1);
});