-- Re-impose NOT NULL only when every issue already has a project.
-- Leftover nulls must be backfilled before this can succeed.
UPDATE issue i
SET project_id = p.id
FROM project p
WHERE i.project_id IS NULL
  AND p.workspace_id = i.workspace_id
  AND p.id = (
      SELECT p2.id
      FROM project p2
      WHERE p2.workspace_id = i.workspace_id
      ORDER BY p2.created_at ASC, p2.id ASC
      LIMIT 1
  );

ALTER TABLE issue ALTER COLUMN project_id SET NOT NULL;
