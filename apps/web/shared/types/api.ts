import type { Issue, IssueStatus, IssuePriority, IssueAssigneeType } from "./issue";
import type { Project } from "./project";
import type { MemberRole } from "./workspace";

// Issue API
export interface CreateIssueRequest {
  title: string;
  description?: string;
  status?: IssueStatus;
  priority?: IssuePriority;
  assignee_type?: IssueAssigneeType;
  assignee_id?: string;
  parent_issue_id?: string;
  due_date?: string;
  project_id?: string;
}

export interface UpdateIssueRequest {
  title?: string;
  description?: string;
  status?: IssueStatus;
  priority?: IssuePriority;
  assignee_type?: IssueAssigneeType | null;
  assignee_id?: string | null;
  position?: number;
  due_date?: string | null;
  project_id?: string;
}

export interface ListIssuesParams {
  limit?: number;
  offset?: number;
  workspace_id?: string;
  status?: IssueStatus;
  priority?: IssuePriority;
  assignee_id?: string;
  project_id?: string;
}

export interface ListIssuesResponse {
  issues: Issue[];
  total: number;
}

export interface CreateProjectRequest {
  name: string;
  position?: number;
}

export interface UpdateProjectRequest {
  name?: string;
  position?: number;
}

export interface ListProjectsResponse {
  projects: Project[];
}

export interface UpdateMeRequest {
  name?: string;
  avatar_url?: string;
}

export interface CreateMemberRequest {
  email: string;
  role?: MemberRole;
  password?: string;
}

export interface UpdateMemberRequest {
  role?: MemberRole;
  name?: string;
  password?: string;
}

// Personal Access Tokens
export interface PersonalAccessToken {
  id: string;
  name: string;
  token_prefix: string;
  expires_at: string | null;
  last_used_at: string | null;
  created_at: string;
}

export interface CreatePersonalAccessTokenRequest {
  name: string;
  expires_in_days?: number;
}

export interface CreatePersonalAccessTokenResponse extends PersonalAccessToken {
  token: string;
}

// Pagination
export interface PaginationParams {
  limit?: number;
  offset?: number;
}
