// @vitest-environment jsdom

/**
 * Date lives in the surface view store like every other live filter. The
 * Filter menu used to hide it unless the caller passed `onDateFilterChange`,
 * so /issues showed Today / Last 3 days while /my-issues (same
 * IssueDisplayControls, no callback) did not — the row appeared and
 * vanished as you switched pages.
 *
 * Save-view still uses IssueFilterMenu without a callback: date presets are
 * relative and are not part of a saved view's query.
 */

import type { ReactElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createStore } from "zustand/vanilla";
import { setApiInstance } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api/client";
import {
  type IssueViewState,
  viewStoreSlice,
} from "@multica/core/issues/stores/view-store";
import { ViewStoreProvider } from "@multica/core/issues/stores/view-store-context";
import { renderWithI18n } from "../../test/i18n";
import { IssueDisplayControls, IssueFilterMenu } from "./issues-header";

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

function renderWithStore(ui: ReactElement) {
  setApiInstance({
    listIssueStatuses: async () => ({ statuses: [], categories: [], total: 0 }),
    listProperties: async () => ({ properties: [] }),
  } as unknown as ApiClient);

  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  });
  const store = createStore<IssueViewState>()(viewStoreSlice);

  return {
    store,
    ...renderWithI18n(
      <QueryClientProvider client={qc}>
        <ViewStoreProvider store={store}>{ui}</ViewStoreProvider>
      </QueryClientProvider>,
    ),
  };
}

async function openFilterMenu() {
  fireEvent.click(screen.getByRole("button", { name: /filter/i }));
  await waitFor(() =>
    expect(screen.getByRole("menuitem", { name: /^Status/ })).toBeInTheDocument(),
  );
}

afterEach(() => {
  cleanup();
  document.body.innerHTML = "";
  vi.restoreAllMocks();
});

describe("IssueDisplayControls date filter", () => {
  it("shows Date with Today / Last 3 days without a caller callback", async () => {
    renderWithStore(<IssueDisplayControls scopedIssues={[]} />);

    await openFilterMenu();

    fireEvent.click(screen.getByRole("menuitem", { name: /^Date\b/ }));
    expect(await screen.findByRole("menuitem", { name: "Today" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Last 3 days" })).toBeInTheDocument();
  });
});

describe("IssueFilterMenu date filter", () => {
  it("hides Date when the host does not own a live date filter", async () => {
    renderWithStore(<IssueFilterMenu trigger={<button type="button">Filter</button>} />);

    await openFilterMenu();

    expect(screen.queryByRole("menuitem", { name: /^Date\b/ })).toBeNull();
  });

  it("defaults the exclusive field radios to Both", async () => {
    const { store } = renderWithStore(
      <IssueFilterMenu
        trigger={<button type="button">Filter</button>}
        onDateFilterChange={(filter) => store.getState().setDateFilter(filter)}
      />,
    );

    await openFilterMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: /^Date\b/ }));

    expect(await screen.findByRole("menuitemradio", { name: "Created" })).toBeInTheDocument();
    expect(screen.getByRole("menuitemradio", { name: "Updated" })).toBeInTheDocument();
    expect(screen.getByRole("menuitemradio", { name: "Both" })).toHaveAttribute(
      "aria-checked",
      "true",
    );

    fireEvent.click(screen.getByRole("menuitem", { name: "Today" }));

    expect(store.getState().dateFilter?.field).toBe("both");
  });
});
