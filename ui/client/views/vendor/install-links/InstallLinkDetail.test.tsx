import { afterEach, expect, mock, test } from "bun:test";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { InstallLinkDetailView } from "./InstallLinkDetail";

const originalFetch = window.fetch;
const originalConfirm = window.confirm;

function renderView(path = "/admin/orgs/org-1/install-links/link-1") {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route
          path="/admin/orgs/:orgId/install-links/:linkId"
          element={<InstallLinkDetailView />}
        />
        <Route
          path="/admin/orgs/:orgId/install-links"
          element={<div>Install Links List</div>}
        />
      </Routes>
    </MemoryRouter>,
  );
}

afterEach(() => {
  Object.defineProperty(window, "fetch", {
    configurable: true,
    value: originalFetch,
  });
  Object.defineProperty(window, "confirm", {
    configurable: true,
    value: originalConfirm,
  });
});

test("loads install link detail and renders pending status", async () => {
  const fetchMock = mock(async (input: RequestInfo | URL) => {
    const url = typeof input === "string" ? input : input.toString();
    expect(url).toBe("/admin/orgs/org-1/install-links-api/link-1");

    return new Response(
      JSON.stringify({
        link: {
          id: "link-1",
          org_id: "org-1",
          app_id: "app-1",
          app_name: "Acme App",
          name: "acme-prod",
          sha: "sha-1",
          used: false,
          created_at: "2026-06-16T00:00:00Z",
        },
        install_url: "https://acme.example.com/install-link?sha=sha-1",
        customer_dashboard_install_url: "",
        nuon_dashboard_install_url: "",
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  });

  Object.defineProperty(window, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  renderView();

  await waitFor(() => {
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  expect(
    screen.getByText("https://acme.example.com/install-link?sha=sha-1"),
  ).toBeInTheDocument();
  expect(screen.getByText("Pending")).toBeInTheDocument();
});

test("deletes install link when delete is confirmed", async () => {
  const fetchMock = mock(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();

    if (init?.method === "DELETE") {
      expect(url).toBe("/admin/orgs/org-1/install-links-api/link-1");
      return new Response(JSON.stringify({ message: "ok" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }

    return new Response(
      JSON.stringify({
        link: {
          id: "link-1",
          org_id: "org-1",
          app_id: "app-1",
          app_name: "Acme App",
          name: "acme-prod",
          sha: "sha-1",
          used: true,
          created_at: "2026-06-16T00:00:00Z",
        },
        install_url: "https://acme.example.com/install-link?sha=sha-1",
        customer_dashboard_install_url: "https://acme.example.com/installs/ins-1",
        nuon_dashboard_install_url: "https://app.nuon.co/org-1/installs/ins-1",
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    );
  });

  Object.defineProperty(window, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  Object.defineProperty(window, "confirm", {
    configurable: true,
    value: () => true,
  });

  renderView();

  const deleteButtons = await screen.findAllByText("Delete Link");

  fireEvent.click(deleteButtons[0]);

  await waitFor(() => {
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
