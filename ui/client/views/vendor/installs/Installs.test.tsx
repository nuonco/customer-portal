import { afterEach, expect, mock, test } from "bun:test";
import { render, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { InstallsView } from "./Installs";

const originalFetch = window.fetch;

function renderView() {
  return render(
    <MemoryRouter initialEntries={["/admin/orgs/org-1/installs"]}>
      <Routes>
        <Route path="/admin/orgs/:orgId/installs" element={<InstallsView />} />
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

test("loads installs once on initial render", async () => {
  const fetchMock = mock(async () =>
    new Response(
      JSON.stringify({ installs: [], org_id: "org-1", nuon_org_id: "nuon-1" }),
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
