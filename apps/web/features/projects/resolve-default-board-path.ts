import { useAuthStore } from "@/features/auth/store";
import { useWorkspaceStore } from "@/features/workspace/store";
import { loadPersonalProjectTabOrder, orderProjectsByPersonalPreference } from "./personal-project-tab-order";
import { useProjectStore } from "./store";

/** True when `next` means the projects hub without a selected project tab. */
function isProjectsIndexPath(next: string): boolean {
  const pathOnly = next.split("?")[0]?.replace(/\/+$/, "") || "/";
  return pathOnly === "/projects";
}

/** When true, wait for project list then use {@link resolveDefaultBoardPath}. */
export function shouldResolveFirstProject(nextSearchParam: string | null): boolean {
  if (!nextSearchParam || !nextSearchParam.startsWith("/")) return true;
  return isProjectsIndexPath(nextSearchParam);
}

/**
 * After workspace + projects are loaded: navigate to `next` when it is a concrete
 * destination; otherwise open the first project board (personal tab order when set).
 */
export function resolveDefaultBoardPath(nextSearchParam: string | null): string {
  if (
    nextSearchParam &&
    nextSearchParam.startsWith("/") &&
    !isProjectsIndexPath(nextSearchParam)
  ) {
    return nextSearchParam;
  }

  const userId = useAuthStore.getState().user?.id ?? "";
  const workspaceId = useWorkspaceStore.getState().workspace?.id ?? "";
  const projects = useProjectStore.getState().projects;
  const ordered = orderProjectsByPersonalPreference(
    projects,
    userId && workspaceId ? loadPersonalProjectTabOrder(userId, workspaceId) : null,
  );
  const first = ordered[0];
  return first ? `/projects/${first.id}` : "/projects";
}
