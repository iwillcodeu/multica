-- Align issue.project_id with application behavior and mainline schema:
-- creates/updates may omit project (workspace Issues "No project").
-- Branch-local 031_project set NOT NULL; main's 034 leaves the column nullable.
ALTER TABLE issue ALTER COLUMN project_id DROP NOT NULL;
