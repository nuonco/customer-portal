import { beforeEach, expect, mock, test } from "bun:test";
import {
  createCustomerInstallWizardInstall,
  getCustomerInstallWizardState,
} from "./install-wizard";

beforeEach(() => {
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      location: new URL("http://localhost:51273/"),
    },
  });

  import.meta.env.VITE_CUSTOMER_SUBDOMAIN = "test-org";

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: mock(async () => new Response("forbidden", { status: 403 })),
  });
});

test("getCustomerInstallWizardState requests wizard JSON", async () => {
  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();

    expect(url).toBe(
      "/bff/portal-api/apps/app-payments/install-wizard?subdomain=test-org&step=stack&install_id=inst_1&workflow_id=wf_1",
    );
    expect(init?.credentials).toBe("include");

    return new Response(
      JSON.stringify({
        app: { app_id: "app-payments", display_name: "Payments", summary: "Deploy the payments stack.", status: "published", logo_light: "", logo_dark: "", platform: "aws" },
        workflow_id: "wf_1",
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  });

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  const result = await getCustomerInstallWizardState({
    appId: "app-payments",
    step: "stack",
    installId: "inst_1",
    workflowId: "wf_1",
  });

  expect(result.app.display_name).toBe("Payments");
  expect(result.workflow_id).toBe("wf_1");
});

test("createCustomerInstallWizardInstall posts wizard install payload", async () => {
  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();
    expect(url).toBe("/bff/portal-api/apps/app-payments/install-wizard?subdomain=test-org");
    expect(init?.method).toBe("POST");
    expect(init?.body).toBe(
      JSON.stringify({ name: "payments-prod", region: "us-east-1", inputs: { env: "prod" } }),
    );

    return new Response(
      JSON.stringify({ install_id: "inst_1", workflow_id: "wf_1", current_step: "stack", next_url: "/apps/app-payments/install?step=stack&install_id=inst_1&workflow_id=wf_1" }),
      { status: 201, headers: { "Content-Type": "application/json" } },
    );
  });

  Object.defineProperty(globalThis, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  const result = await createCustomerInstallWizardInstall("app-payments", {
    name: "payments-prod",
    region: "us-east-1",
    inputs: { env: "prod" },
  });

  expect(result.install_id).toBe("inst_1");
  expect(result.workflow_id).toBe("wf_1");
});
