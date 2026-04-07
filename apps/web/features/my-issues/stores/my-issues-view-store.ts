"use client";

import { createStore, type StoreApi } from "zustand/vanilla";
import { persist } from "zustand/middleware";
import {
  type IssueViewState,
  viewStoreSlice,
  viewStorePersistOptions,
} from "@/features/issues/stores/view-store";

export interface MyIssuesViewState extends IssueViewState {}

const basePersist = viewStorePersistOptions("multica_my_issues_view");

export const myIssuesViewStore: StoreApi<MyIssuesViewState> = createStore<MyIssuesViewState>()(
  persist(
    (set) => ({
      ...viewStoreSlice(set as unknown as StoreApi<IssueViewState>["setState"]),
    }),
    {
      name: basePersist.name,
      partialize: (state: MyIssuesViewState) => ({
        ...basePersist.partialize(state),
      }),
    },
  ),
);
