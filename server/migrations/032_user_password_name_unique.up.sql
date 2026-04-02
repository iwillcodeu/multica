-- Optional password hash for email+password sign-in (bcrypt).
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS password_hash TEXT NULL;

-- Enforce globally unique display names (case-insensitive, trimmed).
-- De-duplicate existing rows: keep first user per normalized name, suffix others.
UPDATE "user" u
SET name = u.name || '-' || left(replace(u.id::text, '-', ''), 8)
WHERE u.id IN (
  SELECT id FROM (
    SELECT id,
           row_number() OVER (
             PARTITION BY lower(btrim(name))
             ORDER BY created_at ASC, id ASC
           ) AS rn
    FROM "user"
  ) t WHERE t.rn > 1
);

CREATE UNIQUE INDEX user_name_lower_unique ON "user" (lower(btrim(name)));
