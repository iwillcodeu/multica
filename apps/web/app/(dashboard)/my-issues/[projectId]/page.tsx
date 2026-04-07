"use client";

import { use, useEffect } from "react";
import { useRouter } from "next/navigation";
import { MyIssuesPage } from "@/features/my-issues";
import { useProjectStore } from "@/features/projects";

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
    if (loading) return;
    if (!projects.some((p) => p.id === projectId)) {
      router.replace("/my-issues");
    }
  }, [loading, projectId, projects, router]);

  return <MyIssuesPage projectId={projectId} />;
}
