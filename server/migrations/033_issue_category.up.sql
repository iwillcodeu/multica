ALTER TABLE issue
    ADD COLUMN category TEXT NOT NULL DEFAULT 'task'
        CHECK (category IN ('bug', 'feature', 'task'));
