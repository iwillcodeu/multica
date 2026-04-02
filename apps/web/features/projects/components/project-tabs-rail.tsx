"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { Plus } from "lucide-react";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { useProjectStore } from "../store";

export function ProjectTabsRail() {
  const pathname = usePathname();
  const router = useRouter();
  const projects = useProjectStore((s) => s.projects);
  const createProject = useProjectStore((s) => s.createProject);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [newName, setNewName] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const activeId =
    pathname.startsWith("/projects/") && pathname.split("/")[2]
      ? pathname.split("/")[2]
      : null;

  const handleCreate = async () => {
    const n = newName.trim();
    if (!n || submitting) return;
    setSubmitting(true);
    try {
      const p = await createProject(n);
      setDialogOpen(false);
      setNewName("");
      router.push(`/projects/${p.id}`);
    } catch {
      toast.error("Failed to create project");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <div
        className="flex w-[52px] shrink-0 flex-col gap-1 border-r border-border/80 bg-muted/20 py-2 px-1"
        aria-label="Projects"
      >
        {projects.map((p) => {
          const active = p.id === activeId;
          return (
            <Tooltip key={p.id}>
              <TooltipTrigger
                render={
                  <Link
                    href={`/projects/${p.id}`}
                    className={cn(
                      "flex min-h-[72px] max-h-28 w-full items-center justify-center rounded-md px-0.5 py-1 text-[11px] font-medium leading-tight transition-colors",
                      "[writing-mode:vertical-rl]",
                      active
                        ? "bg-sidebar-accent text-sidebar-accent-foreground"
                        : "text-muted-foreground hover:bg-accent/80 hover:text-foreground",
                    )}
                  >
                    <span className="line-clamp-4 break-words">{p.name}</span>
                  </Link>
                }
              />
              <TooltipContent side="right">{p.name}</TooltipContent>
            </Tooltip>
          );
        })}
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
