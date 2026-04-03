"use client";

import { memo } from "react";
import Link from "next/link";
import type { Issue } from "@/shared/types";
import { ActorAvatar } from "@/components/common/actor-avatar";
import { useIssueSelectionStore } from "@/features/issues/stores/selection-store";
import { useProjectStore } from "@/features/projects";
import { PriorityIcon } from "./priority-icon";
import { CategoryIcon } from "./category-icon";

function formatDate(date: string): string {
  return new Date(date).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
  });
}

export const ListRow = memo(function ListRow({
  issue,
  showProject,
}: {
  issue: Issue;
  showProject?: boolean;
}) {
  const projects = useProjectStore((s) => s.projects);
  const projectLabel = showProject
    ? projects.find((p) => p.id === issue.project_id)?.name
    : undefined;
  const selected = useIssueSelectionStore((s) => s.selectedIds.has(issue.id));
  const toggle = useIssueSelectionStore((s) => s.toggle);

  return (
    <div
      className={`group/row flex h-9 items-center gap-2 px-4 text-sm transition-colors hover:bg-accent/50 ${
        selected ? "bg-accent/30" : ""
      }`}
    >
      <div className="flex shrink-0 items-center gap-1">
        <div className="flex h-4 w-4 shrink-0 items-center justify-center">
          <input
            type="checkbox"
            checked={selected}
            onChange={() => toggle(issue.id)}
            aria-label={`Select issue ${issue.identifier}`}
            className={`h-3.5 w-3.5 cursor-pointer accent-primary ${
              selected
                ? "opacity-100"
                : "pointer-events-none opacity-0 group-hover/row:pointer-events-auto group-hover/row:opacity-100"
            }`}
          />
        </div>
        <CategoryIcon category={issue.category} className="h-3.5 w-3.5" />
        <PriorityIcon priority={issue.priority} />
      </div>
      <Link
        href={`/issues/${issue.id}`}
        className="flex flex-1 items-center gap-2 min-w-0"
      >
        <span className="w-16 shrink-0 text-xs text-muted-foreground">
          {issue.identifier}
        </span>
        <span className="min-w-0 flex-1 truncate">
          {issue.title}
          {projectLabel && (
            <span className="ml-2 text-xs text-muted-foreground font-normal">
              · {projectLabel}
            </span>
          )}
        </span>
        {issue.due_date && (
          <span className="shrink-0 text-xs text-muted-foreground">
            {formatDate(issue.due_date)}
          </span>
        )}
        {issue.assignee_type && issue.assignee_id && (
          <ActorAvatar
            actorType={issue.assignee_type}
            actorId={issue.assignee_id}
            size={20}
          />
        )}
      </Link>
    </div>
  );
});
