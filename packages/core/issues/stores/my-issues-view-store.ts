"use client";

import { createStore, type StoreApi } from "zustand/vanilla";
import { persist } from "zustand/middleware";
import {
  type IssueViewState,
  viewStoreSlice,
  viewStorePersistOptions,
  mergeViewStatePersisted,
} from "./view-store";
import { registerForWorkspaceRehydration } from "../../platform/workspace-storage";

export type MyIssuesScope = "all" | "assigned" | "created" | "agents";
export type MyIssuesRelation = "all" | "assigned" | "created" | "involved";

// "agents" is the user-facing scope label; the issue APIs call the same
// ownership/squad-membership relation "involved".
export function myIssuesRelationFromScope(
  scope: MyIssuesScope,
): MyIssuesRelation {
  if (scope === "agents") return "involved";
  return scope;
}

export interface MyIssuesViewState extends IssueViewState {
  scope: MyIssuesScope;
  setScope: (next: MyIssuesScope) => void;
}

const basePersist = viewStorePersistOptions("multica_my_issues_view");

const defaultScope: MyIssuesScope = "assigned";

function isMyIssuesScope(v: unknown): v is MyIssuesScope {
  return v === "assigned" || v === "created" || v === "agents";
}

const _myIssuesViewStore = createStore<MyIssuesViewState>()(
  persist(
    (set) => ({
      ...viewStoreSlice(set as unknown as StoreApi<IssueViewState>["setState"]),
      scope: defaultScope,
      setScope: (scope) => set({ scope }),
    }),
    {
      name: basePersist.name,
      storage: basePersist.storage,
      partialize: (state: MyIssuesViewState) => ({
        ...basePersist.partialize(state),
        scope: state.scope,
      }),
        merge: (persisted, current): MyIssuesViewState => {
        const merged = mergeViewStatePersisted<MyIssuesViewState>(persisted, current);
        const rawScope = (persisted as Partial<MyIssuesViewState> | null)?.scope;
        const scope =
          isMyIssuesScope(rawScope)
            ? rawScope
            : isMyIssuesScope(current.scope)
              ? current.scope
              : defaultScope;
        return {
          ...merged,
          scope,
          setScope: current.setScope,
        };
      },
    },
  ),
);

export const myIssuesViewStore: StoreApi<MyIssuesViewState> = _myIssuesViewStore;

registerForWorkspaceRehydration(() => _myIssuesViewStore.persist.rehydrate());
