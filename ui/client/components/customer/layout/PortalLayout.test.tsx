import { afterEach, beforeEach, expect, mock, test } from "bun:test";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Navigate, Route, Routes } from "react-router";
import { CustomerPortalLayout } from "./PortalLayout";
import { CustomerSelectedInstallProvider } from "@/views/customer/install/SelectedInstallProvider";

const originalFetch = window.fetch;

const portalState = {
  user: {
    id: "user-1",
    email: "customer@example.com",
    name: "Customer",
    role: "customer",
  },
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
  active_account: {
    id: "acct-1",
    name: "Core Team",
  },
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
    { id: "ins-1", name: "Install One", app_id: "app-1", app_name: "Payments", status: "", visibility: "", created_at: "", app_logo_light: "", app_logo_dark: "", legacy_base_path: "" },
    { id: "ins-2", name: "Install Two", app_id: "app-1", app_name: "Payments", status: "", visibility: "", created_at: "", app_logo_light: "", app_logo_dark: "", legacy_base_path: "" },
    { id: "ins-3", name: "Install Three", app_id: "app-1", app_name: "Payments", status: "", visibility: "", created_at: "", app_logo_light: "", app_logo_dark: "", legacy_base_path: "" },
    { id: "ins-4", name: "Install Four", app_id: "app-1", app_name: "Payments", status: "", visibility: "", created_at: "", app_logo_light: "", app_logo_dark: "", legacy_base_path: "" },
    { id: "ins-5", name: "Install Five", app_id: "app-1", app_name: "Payments", status: "", visibility: "", created_at: "", app_logo_light: "", app_logo_dark: "", legacy_base_path: "" },
    { id: "ins-6", name: "Install Six", app_id: "app-1", app_name: "Payments", status: "", visibility: "", created_at: "", app_logo_light: "", app_logo_dark: "", legacy_base_path: "" },
    { id: "ins-7", name: "Install Seven", app_id: "app-1", app_name: "Payments", status: "", visibility: "", created_at: "", app_logo_light: "", app_logo_dark: "", legacy_base_path: "" },
  ],
};

function renderLayout(pathname: string) {
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
            <Route path="apps/:appId/install" element={<div>Install wizard</div>} />
            <Route path="installs/:installId">
              <Route index element={<Navigate to="overview" replace />} />
              <Route path=":tab" element={<div>Install detail</div>} />
            </Route>
            <Route path="installs" element={<div>Installs list</div>} />
            <Route path="apps" element={<div>Apps list</div>} />
            <Route path="account" element={<div>Account</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function renderBareInstallLayout(pathname: string) {
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
            <Route path="installs/:installId" element={<CustomerSelectedInstallProvider />} />
            <Route path="installs" element={<div>Installs list</div>} />
            <Route path="apps" element={<div>Apps list</div>} />
            <Route path="account" element={<div>Account</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  document.cookie = "sidebar_minimized=false; path=/";

  const fetchMock = mock(async (input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : input.toString();

    if (url.includes("/portal-api/state")) {
      return new Response(JSON.stringify(portalState), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }

    if (url.includes("/portal-api/installs/") && url.includes("/detail")) {
      return new Response(
        JSON.stringify({
          install: {
            id: "ins-2",
            name: "Install Two",
            status: "",
            visibility: "",
            created_at: "",
            region: "",
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
            stack: { status: "", region: "", account_id: "" },
            sandbox: { status: "", repo: "", branch: "", repo_public: false },
            components: [],
            recent_workflows: [],
            input_fields: [],
          },
          stack: {
            status: "",
            region: "",
            account_id: "",
            vpc: "",
            outputs: {},
            recent_runs: [],
          },
          sandbox: {
            status: "",
            repo: "",
            directory: "",
            branch: "",
            repo_public: false,
            outputs: {},
            recent_runs: [],
          },
          components: {
            items: [],
            deploys: [],
          },
          roles: { roles: [] },
          policies: {
            totals: { pass: 0, warn: 0, deny: 0 },
            items: [],
            reports: [],
          },
          audit: { workflows: [], action_workflows: [] },
          readme: { markdown: "" },
          legacy_base_path: "",
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      );
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
  cleanup();
  document.cookie = "sidebar_minimized=false; path=/";

  Object.defineProperty(window, "fetch", {
    configurable: true,
    value: originalFetch,
  });
});

test("shows install switcher in customer sidebar and filters install list", async () => {
  renderLayout("/installs/ins-2/overview");

  await waitFor(() => {
    expect(screen.getByRole("button", { name: /install two/i })).toBeInTheDocument();
  });

  expect(screen.getByRole("link", { name: "Installs" })).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: /install two/i }));

  const searchInput = await screen.findByPlaceholderText("Search installs...");
  fireEvent.change(searchInput, { target: { value: "seven" } });

  await waitFor(() => {
    expect(screen.getByRole("link", { name: /install seven/i })).toBeInTheDocument();
  });

  expect(screen.queryByRole("link", { name: /install one/i })).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: /create install/i })).toHaveAttribute("href", "/apps/app-1/install");

  fireEvent.click(screen.getByRole("link", { name: /no install selected/i }));

  await waitFor(() => {
    expect(screen.getByText("Installs list")).toBeInTheDocument();
  });
});

test("shows empty install switcher state when no installs exist", async () => {
  const originalInstalls = portalState.installs;
  portalState.installs = [];

  try {
    renderLayout("/installs");

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /no install selected/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /no install selected/i }));

    expect(screen.getByText("No installs found")).toBeInTheDocument();
    expect(screen.queryByPlaceholderText("Search installs...")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: /no install selected/i })).toHaveAttribute("href", "/installs");
    expect(screen.getByRole("link", { name: /create install/i })).toHaveAttribute("href", "/apps/app-1/install");
  } finally {
    portalState.installs = originalInstalls;
  }
});

test("keeps the desktop expand toggle visible after collapsing the sidebar", async () => {
  renderLayout("/installs/ins-2/overview");

  const collapseButton = await screen.findByRole("button", {
    name: /collapse sidebar/i,
  });

  fireEvent.click(collapseButton);

  await waitFor(() => {
    expect(
      screen.getByRole("button", { name: /expand sidebar/i }),
    ).toBeInTheDocument();
  });

  expect(
    screen.getByText("Acme Portal").closest("[aria-hidden='true']"),
  ).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: /expand sidebar/i }));

  await waitFor(() => {
    expect(screen.getByText("Acme Portal")).toBeInTheDocument();
  });
});

test("opens and closes the mobile sidebar", async () => {
  renderLayout("/installs/ins-2/overview");

  await waitFor(() => {
    expect(screen.getByLabelText(/open sidebar/i)).toBeInTheDocument();
  });

  const sidebar = screen.getByRole("complementary");
  expect(sidebar).toHaveAttribute("data-mobile-open", "false");

  fireEvent.click(screen.getByLabelText(/open sidebar/i));

  await waitFor(() => {
    expect(sidebar).toHaveAttribute("data-mobile-open", "true");
  });

  fireEvent.click(screen.getAllByLabelText(/close sidebar/i)[0]);

  await waitFor(() => {
    expect(sidebar).toHaveAttribute("data-mobile-open", "false");
  });
});

test("shows the current install subpage title and active sidebar tab on a bare install route", async () => {
  renderBareInstallLayout("/installs/ins-2");

  await waitFor(() => {
    expect(screen.getByText("Install Two · Overview")).toBeInTheDocument();
  });

  const overviewLink = screen.getByRole("link", { name: "Overview" });
  expect(overviewLink).toHaveClass("bg-black/8");
  expect(overviewLink).toHaveClass("text-text-primary");
});

test("shows the draft install name in the header on the install wizard route", async () => {
  renderLayout("/apps/app-1/install?install_id=ins-2&install_name=my-install");

  await waitFor(() => {
    expect(screen.getByText("Payments")).toBeInTheDocument();
  });

  expect(screen.getByText(/app id:\s*app-1/i)).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /open user menu/i })).toBeInTheDocument();
});

test("shows customer user actions in the topbar dropdown instead of the sidebar panel", async () => {
  renderLayout("/installs/ins-2/overview");

  const userMenuButton = await screen.findByRole("button", {
    name: /open user menu/i,
  });

  expect(screen.queryByPlaceholderText("New group name")).not.toBeInTheDocument();

  fireEvent.click(userMenuButton);

  expect(screen.getByRole("button", { name: /settings/i })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: /log out/i })).toBeInTheDocument();
});
