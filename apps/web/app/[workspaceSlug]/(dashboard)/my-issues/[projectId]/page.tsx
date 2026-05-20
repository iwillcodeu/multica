"use client";

import { use, useEffect } from "react";
import { useRouter } from "next/navigation";
import { MyIssuesPage } from "@multica/views/my-issues";
import { useProjectStore } from "@multica/core/projects";

export default function MyIssuesProjectPage({
  params,
}: {
  params: Promise<{ projectId: string }>;
}) {
  const { projectId } = use(params);
  const router = useRouter();
  const projects = useProjectStore((s) => s.projects);
  const loading = useProjectStore((s) => s.loading);

  useEffect(() => {
    void useProjectStore.getState().fetch();
  }, []);

  useEffect(() => {
    if (loading) return;
    if (!projects.some((p) => p.id === projectId)) {
      router.replace("/my-issues");
    }
  }, [loading, projectId, projects, router]);

  return <MyIssuesPage projectId={projectId} />;
}
