import { Bug, ListTodo, Sparkles } from "lucide-react";
import type { IssueCategory } from "@/shared/types";
import { CATEGORY_CONFIG } from "@/features/issues/config";

const ICONS: Record<IssueCategory, typeof Bug> = {
  bug: Bug,
  feature: Sparkles,
  task: ListTodo,
};

export function CategoryIcon({
  category,
  className = "",
}: {
  category: IssueCategory;
  className?: string;
}) {
  const cfg = CATEGORY_CONFIG[category];
  const Icon = ICONS[category];
  return (
    <Icon
      className={`h-3.5 w-3.5 shrink-0 ${cfg.badgeText} ${className}`}
      aria-hidden
    />
  );
}
