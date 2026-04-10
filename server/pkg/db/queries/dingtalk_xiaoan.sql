-- name: GetDingtalkXiaoanChatMappingByConversationID :one
SELECT * FROM dingtalk_xiaoan_chat_mapping
WHERE conversation_id = $1;

-- name: UpsertDingtalkXiaoanChatMapping :one
INSERT INTO dingtalk_xiaoan_chat_mapping (conversation_id, workspace_id)
VALUES ($1, $2)
ON CONFLICT (conversation_id) DO UPDATE SET workspace_id = EXCLUDED.workspace_id
RETURNING *;

-- name: InsertDingtalkXiaoanDelivery :one
INSERT INTO dingtalk_xiaoan_delivery (
    dingtalk_message_id, conversation_id, workspace_id, status, error_message
) VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetDingtalkXiaoanDeliveryByMessageID :one
SELECT * FROM dingtalk_xiaoan_delivery
WHERE dingtalk_message_id = $1;

-- name: UpdateDingtalkXiaoanDeliveryCreated :one
UPDATE dingtalk_xiaoan_delivery
SET status = 'created',
    issue_id = $2,
    error_message = NULL,
    updated_at = now()
WHERE dingtalk_message_id = $1
RETURNING *;

-- name: UpdateDingtalkXiaoanDeliveryFailed :one
UPDATE dingtalk_xiaoan_delivery
SET status = 'failed',
    error_message = $2,
    updated_at = now()
WHERE dingtalk_message_id = $1
RETURNING *;

-- name: UpdateDingtalkXiaoanDeliveryNeedsEmail :one
UPDATE dingtalk_xiaoan_delivery
SET status = 'needs_email',
    error_message = $2,
    updated_at = now()
WHERE dingtalk_message_id = $1
RETURNING *;

-- name: GetDingtalkXiaoanUserEmailOverride :one
SELECT * FROM dingtalk_xiaoan_user_email_override
WHERE user_id = $1;

-- name: UpsertDingtalkXiaoanUserEmailOverride :one
INSERT INTO dingtalk_xiaoan_user_email_override (user_id, email, source_conversation_id, updated_by_user_id, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (user_id) DO UPDATE
SET email = EXCLUDED.email,
    source_conversation_id = EXCLUDED.source_conversation_id,
    updated_by_user_id = EXCLUDED.updated_by_user_id,
    updated_at = now()
RETURNING *;

-- name: UpsertDingtalkXiaoanPendingIssueCreate :one
INSERT INTO dingtalk_xiaoan_pending_issue_create (
    conversation_id, workspace_id, initiator_message_id, title, project_name, description,
    sender_user_id, assignee_user_id, missing_sender_email, missing_assignee_email, status, expires_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, 'pending', $11, now()
)
ON CONFLICT (conversation_id) DO UPDATE
SET workspace_id = EXCLUDED.workspace_id,
    initiator_message_id = EXCLUDED.initiator_message_id,
    title = EXCLUDED.title,
    project_name = EXCLUDED.project_name,
    description = EXCLUDED.description,
    sender_user_id = EXCLUDED.sender_user_id,
    assignee_user_id = EXCLUDED.assignee_user_id,
    missing_sender_email = EXCLUDED.missing_sender_email,
    missing_assignee_email = EXCLUDED.missing_assignee_email,
    status = 'pending',
    expires_at = EXCLUDED.expires_at,
    updated_at = now()
RETURNING *;

-- name: GetActiveDingtalkXiaoanPendingIssueCreateByConversationID :one
SELECT * FROM dingtalk_xiaoan_pending_issue_create
WHERE conversation_id = $1
  AND status = 'pending'
  AND expires_at > now();

-- name: MarkDingtalkXiaoanPendingIssueCreateCompleted :one
UPDATE dingtalk_xiaoan_pending_issue_create
SET status = 'completed',
    updated_at = now()
WHERE conversation_id = $1
RETURNING *;
