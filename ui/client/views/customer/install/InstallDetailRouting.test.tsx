import { afterEach, beforeEach, expect, mock, test } from "bun:test";
import { fireEvent } from "@testing-library/react";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Navigate, Route, Routes } from "react-router";
import { CustomerPortalLayout } from "@/components/customer/layout/PortalLayout";
import { CustomerSelectedInstallProvider } from "./SelectedInstallProvider";
import { CustomerInstallAuditView } from "./Audit";
import { CustomerInstallOverviewView } from "./Overview";
import { CustomerInstallStackView } from "./Stack";
import { CustomerInstallSandboxView } from "./Sandbox";

const originalFetch = window.fetch;
const originalHTMLIFrameElement = window.HTMLIFrameElement;
const originalGlobalHTMLIFrameElement = globalThis.HTMLIFrameElement;

const portalState = {
  user: { id: "user-1", email: "customer@example.com", name: "Customer", role: "customer" },
  org: {
    id: "org-1",
    name: "Acme",
    subdomain: "acme",
    portal_domain: "acme.localhost",
    admin_url: "https://admin.example.com",
  },
  theme: {
    header_title: "Acme Portal",
    logo_light: "",
    logo_dark: "",
    primary: "#3366ff",
    primary_dark: "#1144cc",
    theme_mode: "auto",
  },
  active_account: { id: "acct-1", name: "Core Team" },
  other_accounts: [],
  apps: [
    {
      id: "app-row-1",
      app_id: "app-1",
      display_name: "Payments",
      status: "published",
      summary: "",
      platform: "",
      logo_light: "",
      logo_dark: "",
      legacy_base_path: "/apps/app-1",
    },
  ],
  installs: [
    {
      id: "ins-1",
      name: "Install One",
      app_id: "app-1",
      app_name: "Payments",
      status: "active",
      visibility: "private",
      created_at: "2026-01-01T00:00:00Z",
      app_logo_light: "",
      app_logo_dark: "",
      legacy_base_path: "/installs/ins-1",
    },
  ],
};

const installDetail = {
  install: {
    id: "ins-1",
    name: "Install One",
    status: "active",
    visibility: "private",
    created_at: "2026-01-01T00:00:00Z",
    region: "us-east-1",
    app_id: "app-1",
    app_name: "Payments",
  },
  app: {
    display_name: "Payments",
    summary: "",
    platform: "",
    logo_light: "",
    logo_dark: "",
  },
  api_deleted_error: false,
  overview: {
    stack: { status: "running", region: "us-east-1", account_id: "123456789012" },
    sandbox: { status: "ready", repo: "nuonco/mono", branch: "main", repo_public: true },
    components: [],
    recent_workflows: [],
    input_fields: [],
  },
  stack: {
    status: "running",
    region: "us-east-1",
    account_id: "123456789012",
    vpc: "",
    outputs: {},
    recent_runs: [
      {
        id: "stack-run-1",
        status: "running",
        version_status: "ready",
        created_at: "2026-01-02T00:00:00Z",
        finished_at: "2026-01-02T01:00:00Z",
      },
    ],
  },
  sandbox: {
    status: "ready",
    repo: "nuonco/mono",
    directory: "",
    branch: "main",
    repo_public: true,
    outputs: {},
    recent_runs: [
      {
        id: "sandbox-run-1",
        status: "completed",
        created_at: "2026-01-03T00:00:00Z",
        finished_at: "2026-01-03T01:00:00Z",
      },
    ],
  },
  components: {
    items: [],
    deploys: [
      {
        id: "deploy-1",
        component_id: "component-1",
        status: "completed",
        created_at: "2026-01-04T00:00:00Z",
        finished_at: "2026-01-04T01:00:00Z",
      },
    ],
  },
  roles: { roles: [] },
  policies: { totals: { pass: 0, warn: 0, deny: 0 }, items: [], reports: [] },
  audit: {
    workflows: [
      {
        id: "workflow-1",
        name: "Deploy",
        status: "completed",
        created_at: "2026-01-05T00:00:00Z",
        finished_at: "2026-01-05T01:00:42Z",
      },
    ],
    action_workflows: [
      {
        id: "action-1",
        name: "Restart",
        status: "completed",
        created_at: "2026-01-06T00:00:00Z",
        finished_at: "2026-01-06T01:00:00Z",
      },
    ],
  },
  readme: { markdown: "" },
  legacy_base_path: "/installs/ins-1",
};

function renderInstallRoute(pathname: string) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[pathname]}>
        <Routes>
          <Route path="/" element={<CustomerPortalLayout />}>
            <Route path="installs/:installId" element={<CustomerSelectedInstallProvider />}>
              <Route index element={<Navigate to="overview" replace />} />
              <Route path="overview" element={<CustomerInstallOverviewView />} />
              <Route path="stack" element={<CustomerInstallStackView />} />
              <Route path="sandbox" element={<CustomerInstallSandboxView />} />
              <Route path="audit" element={<CustomerInstallAuditView />} />
            </Route>
          </Route>
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  const iframeElement =
    window.HTMLIFrameElement ??
    class HTMLIFrameElement extends window.HTMLElement {};

  Object.defineProperty(window, "HTMLIFrameElement", {
    configurable: true,
    value: iframeElement,
  });
  Object.defineProperty(globalThis, "HTMLIFrameElement", {
    configurable: true,
    value: iframeElement,
  });

  const fetchMock = mock(async (input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : input.toString();

    if (url.includes("/portal-api/state")) {
      return new Response(JSON.stringify(portalState), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }

    if (url.includes("/portal-api/installs/ins-1/detail")) {
      return new Response(JSON.stringify(installDetail), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }

    return new Response(JSON.stringify({}), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  });

  Object.defineProperty(window, "fetch", {
    configurable: true,
    value: fetchMock,
  });
});

afterEach(() => {
  Object.defineProperty(window, "HTMLIFrameElement", {
    configurable: true,
    value: originalHTMLIFrameElement,
  });
  Object.defineProperty(globalThis, "HTMLIFrameElement", {
    configurable: true,
    value: originalGlobalHTMLIFrameElement,
  });
  Object.defineProperty(window, "fetch", {
    configurable: true,
    value: originalFetch,
  });
});

test("renders stack route content from dedicated stack view", async () => {
  renderInstallRoute("/installs/ins-1/stack");

  await waitFor(() => {
    expect(screen.getByText("Stack Details")).toBeInTheDocument();
  });
});

test("renders sandbox route content from dedicated sandbox view", async () => {
  renderInstallRoute("/installs/ins-1/sandbox");

  await waitFor(() => {
    expect(screen.getByText("Sandbox Details")).toBeInTheDocument();
  });
});

test("renders audit route content with legacy tab sets", async () => {
  renderInstallRoute("/installs/ins-1/audit");

  await waitFor(() => {
    expect(screen.getByText("Audit Log")).toBeInTheDocument();
  });

  expect(screen.getByText("Duration")).toBeInTheDocument();
  expect(screen.queryByText("Finished")).not.toBeInTheDocument();
  expect(
    screen.getByText((content) => /2026/.test(content) && /\d{1,2}:\d{2}\s?(AM|PM)/i.test(content)),
  ).toBeInTheDocument();
  expect(screen.getByText("1h 42s")).toBeInTheDocument();

  expect(screen.getByRole("button", { name: "Workflows" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Stack" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Sandbox" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Components" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Actions" })).toBeInTheDocument();

  expect(screen.getByText("Deploy")).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "Actions" }));
  await waitFor(() => {
    expect(screen.getByText("Restart")).toBeInTheDocument();
  });

  fireEvent.click(screen.getByRole("button", { name: "Stack" }));
  await waitFor(() => {
    expect(screen.getByText("stack-run-1")).toBeInTheDocument();
  });

  fireEvent.click(screen.getByRole("button", { name: "Sandbox" }));
  await waitFor(() => {
    expect(screen.getByText("sandbox-run-1")).toBeInTheDocument();
  });

  fireEvent.click(screen.getByRole("button", { name: "Components" }));
  await waitFor(() => {
    expect(screen.getByText("deploy-1")).toBeInTheDocument();
  });
});
