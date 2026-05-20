"use client";

import { use } from "react";
import { IssuesPage } from "@multica/views/issues/components";

export default function ProjectBoardPage({
  params,
}: {
  params: Promise<{ projectId: string }>;
}) {
  const { projectId } = use(params);
  return <IssuesPage projectId={projectId} />;
}
