"use client";

import { useMemo } from "react";
import { useAuthStore } from "../auth";
import { useCurrentWorkspace } from "../paths";
import type { Project } from "../types/project";
import {
  loadPersonalProjectTabOrder,
  orderProjectsByPersonalPreference,
} from "./personal-tab-order";

/** Server project list merged with this user's saved tab order for the current workspace (localStorage). */
export function usePersonalProjectTabOrder(projects: Project[]) {
  const userId = useAuthStore((s) => s.user?.id ?? "");
  const workspaceId = useCurrentWorkspace()?.id ?? "";
  return useMemo(
    () =>
      orderProjectsByPersonalPreference(
        projects,
        userId && workspaceId ? loadPersonalProjectTabOrder(userId, workspaceId) : null,
      ),
    [projects, userId, workspaceId],
  );
}
