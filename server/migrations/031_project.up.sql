CREATE TABLE project (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    position DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_project_workspace ON project(workspace_id);

ALTER TABLE issue
    ADD COLUMN project_id UUID REFERENCES project(id) ON DELETE RESTRICT;

INSERT INTO project (workspace_id, name, position)
SELECT id, 'General', 0 FROM workspace;

UPDATE issue i
SET project_id = p.id
FROM project p
WHERE p.workspace_id = i.workspace_id;

ALTER TABLE issue ALTER COLUMN project_id SET NOT NULL;

CREATE INDEX idx_issue_project ON issue(project_id);
