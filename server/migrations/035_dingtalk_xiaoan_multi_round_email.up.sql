-- DingTalk "小安" bot: multi-round email completion (minimal MVP).

CREATE TABLE dingtalk_xiaoan_user_email_override (
    user_id TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    source_conversation_id TEXT,
    updated_by_user_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_dingtalk_xiaoan_user_email_override_email ON dingtalk_xiaoan_user_email_override(email);

-- One active pending create session per conversation (MVP).
CREATE TABLE dingtalk_xiaoan_pending_issue_create (
    conversation_id TEXT PRIMARY KEY,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    initiator_message_id TEXT NOT NULL,
    title TEXT NOT NULL,
    project_name TEXT NOT NULL,
    description TEXT,
    sender_user_id TEXT NOT NULL,
    assignee_user_id TEXT NOT NULL,
    missing_sender_email BOOLEAN NOT NULL DEFAULT false,
    missing_assignee_email BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'pending', -- pending | completed | cancelled | expired
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_dingtalk_xiaoan_pending_issue_create_workspace ON dingtalk_xiaoan_pending_issue_create(workspace_id);
CREATE INDEX idx_dingtalk_xiaoan_pending_issue_create_expires ON dingtalk_xiaoan_pending_issue_create(expires_at);

