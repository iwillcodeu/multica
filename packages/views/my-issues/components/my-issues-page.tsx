"use client";

import { useCallback } from "react";
import { useStore } from "zustand";
import { ListTodo } from "lucide-react";
import { useAuthStore } from "@multica/core/auth";
import {
  myIssuesRelationFromScope,
  myIssuesViewStore,
} from "@multica/core/issues/stores/my-issues-view-store";
import type { Issue } from "@multica/core/types";
import { PageHeader } from "../../layout/page-header";
import { IssueSurface } from "../../issues/surface/issue-surface";
import { useT } from "../../i18n";
import { MyIssuesHeader } from "./my-issues-header";

export function MyIssuesPage({ projectId }: { projectId?: string | null } = {}) {
  const { t } = useT("my-issues");
  const user = useAuthStore((s) => s.user);
  const scope = useStore(myIssuesViewStore, (s) => s.scope);
  const setScope = useStore(myIssuesViewStore, (s) => s.setScope);
  const clientFilter = useCallback(
    (issue: Issue) => (projectId ? issue.project_id === projectId : true),
    [projectId],
  );

  return (
    <div className="flex flex-1 min-h-0 flex-col">
      <PageHeader>
        <ListTodo className="h-4 w-4 text-muted-foreground" />
        <h1 className="text-body font-medium">{t(($) => $.page.breadcrumb)}</h1>
      </PageHeader>

      {user ? (
        <IssueSurface
          scope={{
            type: "my",
            userId: user.id,
            relation: myIssuesRelationFromScope(scope),
          }}
          modes={["board", "list", "table", "swimlane"]}
          batchToolbar="list"
          createDefaults={projectId ? { project_id: projectId } : undefined}
          clientFilter={projectId ? clientFilter : undefined}
          renderHeader={({ controller }) => (
            <MyIssuesHeader
              allIssues={controller.surfaceIssues}
              workingAgents={controller.workingAgents}
              scope={scope}
              onScopeChange={setScope}
              isRefreshing={controller.isRefreshing}
              facetCountsExact={controller.facetCountsExact}
              tableFacetCounts={controller.tableFacetCounts}
              onTableFacetChange={controller.setActiveTableFacet}
            />
          )}
          renderEmpty={() => (
            <div className="flex flex-1 min-h-0 flex-col items-center justify-center gap-2 text-muted-foreground">
              <ListTodo className="h-10 w-10 text-faint-foreground" />
              <p className="text-body">{t(($) => $.page.empty_title)}</p>
              <p className="text-caption">{t(($) => $.page.empty_description)}</p>
            </div>
          )}
        />
      ) : null}
    </div>
  );
}
