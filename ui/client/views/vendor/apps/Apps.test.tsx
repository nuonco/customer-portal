import { afterEach, expect, mock, test } from "bun:test";
import { render, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { AppsView } from "./Apps";

const originalFetch = window.fetch;

function renderView() {
  return render(
    <MemoryRouter initialEntries={["/admin/orgs/org-1/apps"]}>
      <Routes>
        <Route path="/admin/orgs/:orgId/apps" element={<AppsView />} />
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

test("loads app catalog once on initial render", async () => {
  const fetchMock = mock(async () =>
    new Response(
      JSON.stringify({
        apps: [],
        pagination: {
          current_page: 1,
          has_previous: false,
          has_next: false,
          previous_page: 0,
          next_page: 2,
        },
      }),
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