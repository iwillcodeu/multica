import type { IssueCategory } from "@/shared/types";

export const ISSUE_CATEGORIES: IssueCategory[] = ["bug", "feature", "task"];

export const CATEGORY_CONFIG: Record<
  IssueCategory,
  { label: string; badgeBg: string; badgeText: string }
> = {
  bug: {
    label: "Bug",
    badgeBg: "bg-destructive/15",
    badgeText: "text-destructive",
  },
  feature: {
    label: "Feature",
    badgeBg: "bg-primary/15",
    badgeText: "text-primary",
  },
  task: {
    label: "Task",
    badgeBg: "bg-muted",
    badgeText: "text-muted-foreground",
  },
};
