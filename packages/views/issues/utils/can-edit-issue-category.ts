import type { Issue, MemberRole } from "@/shared/types";

/** Whether the current user may change an issue's category (backend enforces the same rules). */
export function canEditIssueCategory(
  issue: Issue,
  currentUserId: string | undefined,
  memberRole: MemberRole | undefined,
): boolean {
  if (!currentUserId) return false;
  if (memberRole === "owner" || memberRole === "admin") return true;
  if (issue.creator_type === "member" && issue.creator_id === currentUserId) return true;
  if (issue.assignee_type === "member" && issue.assignee_id === currentUserId) return true;
  return false;
}
