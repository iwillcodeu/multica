"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { MulticaIcon } from "@/components/multica-icon";
import { Button } from "@/components/ui/button";
import { useProjectStore } from "@/features/projects";

export default function ProjectsIndexPage() {
  const router = useRouter();
  const projects = useProjectStore((s) => s.projects);
  const loading = useProjectStore((s) => s.loading);
  const createProject = useProjectStore((s) => s.createProject);

  useEffect(() => {
    if (loading) return;
    if (projects.length > 0) {
      router.replace(`/projects/${projects[0]!.id}`);
    }
  }, [loading, projects, router]);

  if (loading) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <MulticaIcon className="size-6 animate-pulse" />
      </div>
    );
  }

  if (projects.length > 0) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <MulticaIcon className="size-6 animate-pulse" />
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-4 p-6 text-center">
      <p className="text-sm text-muted-foreground max-w-sm">
        No projects yet. Create one to start tracking issues on a board.
      </p>
      <Button
        type="button"
        onClick={async () => {
          const p = await createProject("General");
          router.replace(`/projects/${p.id}`);
        }}
      >
        Create &quot;General&quot; project
      </Button>
    </div>
  );
}
