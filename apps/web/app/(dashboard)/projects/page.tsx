"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { MulticaIcon } from "@multica/ui/components/common/multica-icon";
import { Button } from "@multica/ui/components/ui/button";
import { canCreateOrRenameProjects, useCurrentWorkspaceMember } from "@multica/core/workspace/hooks";
import { usePersonalProjectTabOrder, useProjectStore } from "@/features/projects";

export default function ProjectsIndexPage() {
  const router = useRouter();
  const projects = useProjectStore((s) => s.projects);
  const loading = useProjectStore((s) => s.loading);
  const createProject = useProjectStore((s) => s.createProject);
  const member = useCurrentWorkspaceMember();
  const canCreate = canCreateOrRenameProjects(member?.role);
  const orderedProjects = usePersonalProjectTabOrder(projects);

  useEffect(() => {
    void useProjectStore.getState().fetch();
  }, []);

  useEffect(() => {
    if (loading) return;
    if (orderedProjects.length > 0) {
      router.replace(`/projects/${orderedProjects[0]!.id}`);
    }
  }, [loading, orderedProjects, router]);

  if (loading) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <MulticaIcon className="size-6 animate-pulse" />
      </div>
    );
  }

  if (orderedProjects.length > 0) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <MulticaIcon className="size-6 animate-pulse" />
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-4 p-6 text-center">
      <p className="text-sm text-muted-foreground max-w-sm">
        {canCreate
          ? "No projects yet. Create one to start tracking issues on a board."
          : "No projects in this workspace. Ask a workspace owner or admin to create a project."}
      </p>
      {canCreate && (
        <Button
          type="button"
          onClick={async () => {
            const p = await createProject("General");
            router.replace(`/projects/${p.id}`);
          }}
        >
          Create &quot;General&quot; project
        </Button>
      )}
    </div>
  );
}
