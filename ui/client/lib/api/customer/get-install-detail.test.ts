import { beforeEach, expect, mock, test } from "bun:test";
import { getCustomerInstallDetail } from "./get-install-detail";

beforeEach(() => {
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      location: new URL("http://localhost:8080/"),
    },
  });

  import.meta.env.VITE_CUSTOMER_SUBDOMAIN = "test-org";

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: mock(async () => new Response("forbidden", { status: 403 })),
  });
});

test("getCustomerInstallDetail requests install detail JSON", async () => {
  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();

    expect(url).toBe("/portal-api/installs/inst_1/detail?subdomain=test-org");
    expect(init?.credentials).toBe("include");

    return new Response(
      JSON.stringify({
        install: {
          id: "inst_1",
          name: "Payments Prod",
          status: "active",
          visibility: "account",
          created_at: "2026-07-08T00:00:00Z",
          region: "us-east-1",
          app_id: "app-payments",
          app_name: "Payments",
        },
        app: {
          display_name: "Payments",
          summary: "Deploy the payments stack.",
          platform: "aws",
          logo_light: "",
          logo_dark: "",
        },
        api_deleted_error: false,
        overview: {
          stack: { status: "ready", region: "us-east-1", account_id: "123" },
          sandbox: { status: "ready", repo: "acme/payments", branch: "main", repo_public: false },
          components: [],
          recent_workflows: [],
          input_fields: [],
        },
        stack: { status: "ready", region: "us-east-1", account_id: "123", vpc: "vpc-1", outputs: {}, recent_runs: [] },
        sandbox: { status: "ready", repo: "acme/payments", directory: "/", branch: "main", repo_public: false, outputs: {}, recent_runs: [] },
        components: { items: [], deploys: [] },
        roles: { roles: [] },
        policies: { totals: { pass: 0, warn: 0, deny: 0 }, items: [], reports: [] },
        audit: { workflows: [], action_workflows: [] },
        readme: { markdown: "# Payments" },
        legacy_base_path: "/installs/inst_1",
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  });

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  const result = await getCustomerInstallDetail("inst_1");

  expect(result.install.name).toBe("Payments Prod");
  expect(result.overview.stack.status).toBe("ready");
  expect(result.audit.action_workflows).toEqual([]);
});
