import { afterEach, expect, mock, test } from "bun:test";
import { render, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { AccountsView } from "./Accounts";

const originalFetch = window.fetch;

function renderView() {
  return render(
    <MemoryRouter initialEntries={["/admin/orgs/org-1/accounts"]}>
      <Routes>
        <Route
          path="/admin/orgs/:orgId/accounts"
          element={<AccountsView />}
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
});

test("loads accounts once on initial render", async () => {
  const fetchMock = mock(async () =>
    new Response(
      JSON.stringify({ accounts: [], org_id: "org-1" }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ),
  );

  Object.defineProperty(window, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  renderView();

  await waitFor(() => {
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  await new Promise((resolve) => setTimeout(resolve, 500));

  expect(fetchMock).toHaveBeenCalledTimes(1);
});

test("renders account rows when data is returned", async () => {
  const fetchMock = mock(async () =>
    new Response(
      JSON.stringify({
        accounts: [
          {
            id: "acc-1",
            name: "Acme Corp",
            member_count: 3,
            install_count: 2,
            created_at: "2024-01-15T00:00:00Z",
          },
        ],
        org_id: "org-1",
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    ),
  );

  Object.defineProperty(window, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  const { getByText } = renderView();

  await waitFor(() => {
    expect(getByText("Acme Corp")).toBeTruthy();
  });
});

test("renders error message on fetch failure", async () => {
  const fetchMock = mock(
    async () => new Response("error", { status: 500 }),
  );

  Object.defineProperty(window, "fetch", {
    configurable: true,
    value: fetchMock,
  });

  const { getByText } = renderView();

  await waitFor(() => {
    expect(getByText("Unable to load user groups right now.")).toBeTruthy();
  });
});
