"use client";

import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "../hooks";
import { useCurrentMember } from "../permissions/use-current-member";
import { useCurrentWorkspace } from "../paths";
import type { MemberRole, MemberWithUser } from "../types";
import { memberListOptions, agentListOptions } from "./queries";

/** Current user's membership in the active workspace, if any. */
export function useCurrentWorkspaceMember(): MemberWithUser | null {
  const wsId = useCurrentWorkspace()?.id ?? "";
  const { member } = useCurrentMember(wsId);
  return member;
}

export function canCreateOrRenameProjects(role: MemberRole | undefined): boolean {
  return role === "owner" || role === "admin";
}

export function canDeleteProjects(role: MemberRole | undefined): boolean {
  return role === "owner";
}

export function useActorName() {
  const wsId = useWorkspaceId();
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const { data: agents = [] } = useQuery(agentListOptions(wsId));

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
