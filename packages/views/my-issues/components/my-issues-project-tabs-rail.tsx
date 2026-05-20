"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  DndContext,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  arrayMove,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { Plus } from "lucide-react";
import { toast } from "sonner";
import { cn } from "@multica/ui/lib/utils";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@multica/ui/components/ui/dialog";
import { Input } from "@multica/ui/components/ui/input";
import { Tooltip, TooltipTrigger, TooltipContent } from "@multica/ui/components/ui/tooltip";
import { useAuthStore } from "@multica/core/auth";
import {
  canCreateOrRenameProjects,
  useCurrentWorkspaceMember,
} from "@multica/core/workspace/hooks";
import { useCurrentWorkspace } from "@multica/core/paths";
import type { Project } from "@multica/core/types/project";
import {
  savePersonalProjectTabOrder,
  usePersonalProjectTabOrder,
  useProjectStore,
} from "@multica/core/projects";
import { AppLink, useNavigation } from "../../navigation";

function SortableProjectTab({
  project,
  active,
}: {
  project: Project;
  active: boolean;
}) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: project.id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className="touch-none"
      {...attributes}
      {...listeners}
    >
      <Tooltip>
        <TooltipTrigger
          render={
            <AppLink
              href={`/my-issues/${project.id}`}
              className={cn(
                "flex min-h-[72px] max-h-28 w-full items-center justify-center rounded-full px-0.5 py-1 text-[11px] font-medium leading-tight transition-colors",
                "[writing-mode:vertical-rl]",
                active
                  ? "bg-sidebar-accent text-sidebar-accent-foreground"
                  : "text-muted-foreground hover:bg-sidebar-item-hover hover:text-foreground",
                isDragging && "pointer-events-none opacity-60",
              )}
            >
              <span className="line-clamp-4 break-words">{project.title}</span>
            </AppLink>
          }
        />
        <TooltipContent side="right">{project.title}</TooltipContent>
      </Tooltip>
    </div>
  );
}

export function MyIssuesProjectTabsRail() {
  const { push, pathname } = useNavigation();
  const workspace = useCurrentWorkspace();
  const workspaceId = workspace?.id ?? "";

  useEffect(() => {
    void useProjectStore.getState().fetch();
  }, []);

  const projects = useProjectStore((s) => s.projects);
  const createProject = useProjectStore((s) => s.createProject);
  const member = useCurrentWorkspaceMember();
  const canAddProject = canCreateOrRenameProjects(member?.role);
  const userId = useAuthStore((s) => s.user?.id ?? "");

  const [dialogOpen, setDialogOpen] = useState(false);
  const [newName, setNewName] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const mergedProjects = usePersonalProjectTabOrder(projects);

  const [orderedProjects, setOrderedProjects] = useState(mergedProjects);
  useEffect(() => {
    setOrderedProjects(mergedProjects);
  }, [mergedProjects]);

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 6 },
    }),
  );

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      const { active, over } = event;
      if (!over || active.id === over.id) return;
      const oldIndex = orderedProjects.findIndex((p) => p.id === active.id);
      const newIndex = orderedProjects.findIndex((p) => p.id === over.id);
      if (oldIndex < 0 || newIndex < 0) return;
      const next = arrayMove(orderedProjects, oldIndex, newIndex);
      setOrderedProjects(next);
      if (userId && workspaceId) {
        savePersonalProjectTabOrder(
          userId,
          workspaceId,
          next.map((p) => p.id),
        );
      }
    },
    [orderedProjects, userId, workspaceId],
  );

  const { allActive, projectActiveId } = useMemo(() => {
    const parts = pathname.split("/").filter(Boolean);
    const mi = parts.indexOf("my-issues");
    if (mi < 0) {
      return { allActive: false, projectActiveId: null as string | null };
    }
    const after = parts[mi + 1];
    if (!after) {
      return { allActive: true, projectActiveId: null as string | null };
    }
    if (projects.some((p) => p.id === after)) {
      return { allActive: false, projectActiveId: after };
    }
    return { allActive: true, projectActiveId: null as string | null };
  }, [pathname, projects]);

  const handleCreate = async () => {
    const n = newName.trim();
    if (!n || submitting) return;
    setSubmitting(true);
    try {
      const p = await createProject(n);
      setDialogOpen(false);
      setNewName("");
      push(`/my-issues/${p.id}`);
    } catch {
      toast.error("Failed to create project");
    } finally {
      setSubmitting(false);
    }
  };

  const sortableIds = orderedProjects.map((p) => p.id);

  return (
    <>
      <div
        className="flex w-[52px] shrink-0 flex-col gap-1 border-r border-border/80 bg-muted/20 py-2 px-1"
        aria-label="Projects"
      >
        <Tooltip>
          <TooltipTrigger
            render={
              <AppLink
                href="/my-issues"
                className={cn(
                  "flex min-h-[72px] max-h-28 w-full items-center justify-center rounded-full px-0.5 py-1 text-[11px] font-medium leading-tight transition-colors",
                  "[writing-mode:vertical-rl]",
                  allActive
                    ? "bg-sidebar-accent text-sidebar-accent-foreground"
                    : "text-muted-foreground hover:bg-sidebar-item-hover hover:text-foreground",
                )}
              >
                <span className="line-clamp-4 break-words">All</span>
              </AppLink>
            }
          />
          <TooltipContent side="right">All projects</TooltipContent>
        </Tooltip>

        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={handleDragEnd}
        >
          <SortableContext items={sortableIds} strategy={verticalListSortingStrategy}>
            {orderedProjects.map((p) => (
              <SortableProjectTab key={p.id} project={p} active={p.id === projectActiveId} />
            ))}
          </SortableContext>
        </DndContext>
        {canAddProject && (
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-9 shrink-0 text-muted-foreground"
                  onClick={() => setDialogOpen(true)}
                >
                  <Plus className="size-4" />
                </Button>
              }
            />
            <TooltipContent side="right">New project</TooltipContent>
          </Tooltip>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>New project</DialogTitle>
          </DialogHeader>
          <Input
            placeholder="Project name"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void handleCreate();
            }}
            autoFocus
          />
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              type="button"
              onClick={() => void handleCreate()}
              disabled={!newName.trim() || submitting}
            >
              {submitting ? "Creating…" : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
