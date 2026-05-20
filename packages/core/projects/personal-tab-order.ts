import type { Project } from "../types/project";
import { defaultStorage } from "../platform/storage";

const STORAGE_PREFIX = "multica.projectTabOrder.v1";

function storageKey(userId: string, workspaceId: string): string {
  return `${STORAGE_PREFIX}:${userId}:${workspaceId}`;
}

function defaultSort(a: Project, b: Project): number {
  return a.title.localeCompare(b.title);
}

/** Merge API projects with a per-user saved id order; unknown ids append sorted by title. */
export function orderProjectsByPersonalPreference(
  projects: Project[],
  savedIds: string[] | null | undefined,
): Project[] {
  if (!savedIds?.length) {
    return [...projects].sort(defaultSort);
  }
  const byId = new Map(projects.map((p) => [p.id, p]));
  const ordered: Project[] = [];
  for (const id of savedIds) {
    const p = byId.get(id);
    if (p) {
      ordered.push(p);
      byId.delete(id);
    }
  }
  const rest = [...byId.values()].sort(defaultSort);
  return [...ordered, ...rest];
}

export function loadPersonalProjectTabOrder(
  userId: string,
  workspaceId: string,
): string[] | null {
  if (typeof window === "undefined" || !userId || !workspaceId) return null;
  try {
    const raw = defaultStorage.getItem(storageKey(userId, workspaceId));
    if (!raw) return null;
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return null;
    return parsed.filter((x): x is string => typeof x === "string");
  } catch {
    return null;
  }
}

export function savePersonalProjectTabOrder(
  userId: string,
  workspaceId: string,
  orderedIds: string[],
): void {
  if (typeof window === "undefined" || !userId || !workspaceId) return;
  try {
    defaultStorage.setItem(storageKey(userId, workspaceId), JSON.stringify(orderedIds));
  } catch {
    // private mode / quota
  }
}
