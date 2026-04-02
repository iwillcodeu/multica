"use client";

import { useMemo } from "react";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import type { Project } from "@/shared/types";
import { loadPersonalProjectTabOrder, orderProjectsByPersonalPreference } from "./personal-project-tab-order";

/** Server project list merged with this user's saved tab order for the current workspace (localStorage). */
export function usePersonalProjectTabOrder(projects: Project[]) {
  const userId = useAuthStore((s) => s.user?.id ?? "");
  const workspaceId = useWorkspaceStore((s) => s.workspace?.id ?? "");
  return useMemo(
    () =>
      orderProjectsByPersonalPreference(
        projects,
        userId && workspaceId ? loadPersonalProjectTabOrder(userId, workspaceId) : null,
      ),
    [projects, userId, workspaceId],
  );
}
