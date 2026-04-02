"use client";

import { useMemo } from "react";
import type { MemberRole, MemberWithUser } from "@/shared/types";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "./store";

/** Current user's membership in the active workspace, if any. */
export function useCurrentWorkspaceMember(): MemberWithUser | null {
  const userId = useAuthStore((s) => s.user?.id);
  const members = useWorkspaceStore((s) => s.members);
  return useMemo(
    () => members.find((m) => m.user_id === userId) ?? null,
    [members, userId],
  );
}

export function canCreateOrRenameProjects(role: MemberRole | undefined): boolean {
  return role === "owner" || role === "admin";
}

export function canDeleteProjects(role: MemberRole | undefined): boolean {
  return role === "owner";
}

export function useActorName() {
  const members = useWorkspaceStore((s) => s.members);
  const agents = useWorkspaceStore((s) => s.agents);

  const getMemberName = (userId: string) => {
    const m = members.find((m) => m.user_id === userId);
    return m?.name ?? "Unknown";
  };

  const getAgentName = (agentId: string) => {
    const a = agents.find((a) => a.id === agentId);
    return a?.name ?? "Unknown Agent";
  };

  const getActorName = (type: string, id: string) => {
    if (type === "member") return getMemberName(id);
    if (type === "agent") return getAgentName(id);
    return "System";
  };

  const getActorInitials = (type: string, id: string) => {
    const name = getActorName(type, id);
    return name
      .split(" ")
      .map((w) => w[0])
      .join("")
      .toUpperCase()
      .slice(0, 2);
  };

  const getActorAvatarUrl = (type: string, id: string): string | null => {
    if (type === "member") return members.find((m) => m.user_id === id)?.avatar_url ?? null;
    if (type === "agent") return agents.find((a) => a.id === id)?.avatar_url ?? null;
    return null;
  };

  return { getMemberName, getAgentName, getActorName, getActorInitials, getActorAvatarUrl };
}
