-- Modern project columns + issue.project_id wiring.
--
-- Historically both 031_project and this migration could create/evolve project.
-- On DBs where 031 already ran, a plain CREATE TABLE conflicts. This file is
-- idempotent: create-if-missing, then ALTER only what is missing, and migrate
-- the legacy `name` column to `title` when needed.

CREATE TABLE IF NOT EXISTS project (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    icon TEXT,
    status TEXT NOT NULL DEFAULT 'planned'
        CHECK (status IN ('planned', 'in_progress', 'paused', 'completed', 'cancelled')),
    lead_type TEXT CHECK (lead_type IN ('member', 'agent')),
    lead_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_project_workspace ON project(workspace_id);

-- Parseable for sqlc (it ignores the DO-block below). IF NOT EXISTS is a
-- no-op on databases that already ran this migration's procedural upgrades.
ALTER TABLE project ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE project ADD COLUMN IF NOT EXISTS icon TEXT;
ALTER TABLE project ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'planned';
ALTER TABLE project ADD COLUMN IF NOT EXISTS lead_type TEXT;
ALTER TABLE project ADD COLUMN IF NOT EXISTS lead_id UUID;

ALTER TABLE issue ADD COLUMN IF NOT EXISTS project_id UUID REFERENCES project(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_issue_project ON issue(project_id);

-- Upgrade databases that came from the older project shape (031_project uses `name`, not `title`).
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'project' AND column_name = 'name'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'project' AND column_name = 'title'
    ) THEN
        EXECUTE 'ALTER TABLE project RENAME COLUMN name TO title';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'project' AND column_name = 'description'
    ) THEN
        EXECUTE 'ALTER TABLE project ADD COLUMN description TEXT';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'project' AND column_name = 'icon'
    ) THEN
        EXECUTE 'ALTER TABLE project ADD COLUMN icon TEXT';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'project' AND column_name = 'status'
    ) THEN
        EXECUTE $sql$
            ALTER TABLE project ADD COLUMN status TEXT NOT NULL DEFAULT 'planned'
                CHECK (status IN ('planned', 'in_progress', 'paused', 'completed', 'cancelled'))
        $sql$;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'project' AND column_name = 'lead_type'
    ) THEN
        EXECUTE $lt$ALTER TABLE project ADD COLUMN lead_type TEXT CHECK (lead_type IN ('member', 'agent'))$lt$;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'project' AND column_name = 'lead_id'
    ) THEN
        EXECUTE 'ALTER TABLE project ADD COLUMN lead_id UUID';
    END IF;

    -- Ensure NOT NULL constraint on title if the column exists but allows NULL from legacy quirks.
    IF EXISTS (
        SELECT 1 FROM information_schema.columns c
        WHERE c.table_schema = 'public'
          AND c.table_name = 'project'
          AND c.column_name = 'title'
          AND c.is_nullable = 'YES'
    ) THEN
        EXECUTE $ttl$UPDATE project SET title = 'General' WHERE title IS NULL OR trim(title) = ''$ttl$;
        EXECUTE $tnn$ALTER TABLE project ALTER COLUMN title SET NOT NULL$tnn$;
    END IF;
END
$$;
