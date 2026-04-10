-- DingTalk "小安" bot: chat -> workspace routing and message idempotency.

CREATE TABLE dingtalk_xiaoan_chat_mapping (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id TEXT NOT NULL UNIQUE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_dingtalk_xiaoan_chat_mapping_workspace ON dingtalk_xiaoan_chat_mapping(workspace_id);

CREATE TABLE dingtalk_xiaoan_delivery (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dingtalk_message_id TEXT NOT NULL UNIQUE,
    conversation_id TEXT NOT NULL,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    issue_id UUID REFERENCES issue(id) ON DELETE SET NULL,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_dingtalk_xiaoan_delivery_workspace ON dingtalk_xiaoan_delivery(workspace_id);
