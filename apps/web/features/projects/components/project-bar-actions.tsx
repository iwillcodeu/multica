"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Pencil, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Input } from "@/components/ui/input";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import {
  canCreateOrRenameProjects,
  canDeleteProjects,
  useCurrentWorkspaceMember,
} from "@/features/workspace";
import { useProjectStore } from "../store";

export function ProjectBarActions({ projectId }: { projectId: string }) {
  const router = useRouter();
  const member = useCurrentWorkspaceMember();
  const role = member?.role;
  const projects = useProjectStore((s) => s.projects);
  const updateProject = useProjectStore((s) => s.updateProject);
  const deleteProject = useProjectStore((s) => s.deleteProject);

  const project = projects.find((p) => p.id === projectId);
  const canRename = canCreateOrRenameProjects(role);
  const canDelete = canDeleteProjects(role);

  const [renameOpen, setRenameOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [name, setName] = useState(project?.name ?? "");
  const [busy, setBusy] = useState(false);

  if (!canRename && !canDelete) return null;

  const openRename = () => {
    setName(project?.name ?? "");
    setRenameOpen(true);
  };

  const handleRename = async () => {
    const n = name.trim();
    if (!n || busy || n === project?.name) {
      setRenameOpen(false);
      return;
    }
    setBusy(true);
    try {
      await updateProject(projectId, { name: n });
      toast.success("Project renamed");
      setRenameOpen(false);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to rename project");
    } finally {
      setBusy(false);
    }
  };

  const handleDelete = async () => {
    if (busy) return;
    setBusy(true);
    try {
      await deleteProject(projectId);
      toast.success("Project deleted");
      setDeleteOpen(false);
      const next = useProjectStore.getState().projects;
      if (next[0]) {
        router.replace(`/projects/${next[0].id}`);
      } else {
        router.replace("/projects");
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to delete project");
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <div className="flex shrink-0 items-center gap-1">
        {canRename && (
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="text-muted-foreground"
                  aria-label="Rename project"
                  onClick={openRename}
                >
                  <Pencil className="size-4" />
                </Button>
              }
            />
            <TooltipContent side="bottom">Rename project</TooltipContent>
          </Tooltip>
        )}
        {canDelete && (
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="text-muted-foreground hover:text-destructive"
                  aria-label="Delete project"
                  onClick={() => setDeleteOpen(true)}
                >
                  <Trash2 className="size-4" />
                </Button>
              }
            />
            <TooltipContent side="bottom">Delete project</TooltipContent>
          </Tooltip>
        )}
      </div>

      <Dialog
        open={renameOpen}
        onOpenChange={(o) => {
          setRenameOpen(o);
          if (o) setName(project?.name ?? "");
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Rename project</DialogTitle>
          </DialogHeader>
          <Input
            placeholder="Project name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void handleRename();
            }}
            autoFocus
          />
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setRenameOpen(false)}>
              Cancel
            </Button>
            <Button
              type="button"
              onClick={() => void handleRename()}
              disabled={!name.trim() || name.trim() === project?.name || busy}
            >
              {busy ? "Saving…" : "Save"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete project?</AlertDialogTitle>
            <AlertDialogDescription>
              This removes the project from the workspace. You can only delete a project that has no
              issues and is not the last project in the workspace.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={(ev) => {
                ev.preventDefault();
                void handleDelete();
              }}
              disabled={busy}
            >
              {busy ? "Deleting…" : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
