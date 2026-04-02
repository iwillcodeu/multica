"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { MulticaIcon } from "@/components/multica-icon";
import { useProjectStore } from "@/features/projects";

export default function LegacyIssuesRedirectPage() {
  const router = useRouter();
  const projects = useProjectStore((s) => s.projects);
  const loading = useProjectStore((s) => s.loading);

  useEffect(() => {
    if (loading) return;
    if (projects.length > 0) {
      router.replace(`/projects/${projects[0]!.id}`);
      return;
    }
    router.replace("/projects");
  }, [loading, projects, router]);

  return (
    <div className="flex flex-1 items-center justify-center">
      <MulticaIcon className="size-6 animate-pulse" />
    </div>
  );
}
