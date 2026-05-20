package db

import "github.com/jackc/pgx/v5/pgtype"

// DingTalk "小安" row types consumed by dingtalk_xiaoan.sql.go.
// Keep in sync with migrations 034 / 035. Not emitted by sqlc because
// running sqlc against migrations/ fails on duplicate DDL across versioned files.

type DingtalkXiaoanChatMapping struct {
	ID             pgtype.UUID        `json:"id"`
	ConversationID string             `json:"conversation_id"`
	WorkspaceID    pgtype.UUID        `json:"workspace_id"`
	CreatedAt      pgtype.Timestamptz `json:"created_at"`
}

type DingtalkXiaoanDelivery struct {
	ID                pgtype.UUID        `json:"id"`
	DingtalkMessageID string             `json:"dingtalk_message_id"`
	ConversationID    string             `json:"conversation_id"`
	WorkspaceID     pgtype.UUID        `json:"workspace_id"`
	Status            string             `json:"status"`
	IssueID           pgtype.UUID        `json:"issue_id"`
	ErrorMessage      pgtype.Text        `json:"error_message"`
	CreatedAt         pgtype.Timestamptz `json:"created_at"`
	UpdatedAt         pgtype.Timestamptz `json:"updated_at"`
}

type DingtalkXiaoanUserEmailOverride struct {
	UserID               string             `json:"user_id"`
	Email                string             `json:"email"`
	SourceConversationID pgtype.Text        `json:"source_conversation_id"`
	UpdatedByUserID      pgtype.Text        `json:"updated_by_user_id"`
	CreatedAt            pgtype.Timestamptz `json:"created_at"`
	UpdatedAt            pgtype.Timestamptz `json:"updated_at"`
}

type DingtalkXiaoanPendingIssueCreate struct {
	ConversationID       string             `json:"conversation_id"`
	WorkspaceID          pgtype.UUID        `json:"workspace_id"`
	InitiatorMessageID   string             `json:"initiator_message_id"`
	Title                string             `json:"title"`
	ProjectName          string             `json:"project_name"`
	Description          pgtype.Text        `json:"description"`
	SenderUserID         string             `json:"sender_user_id"`
	AssigneeUserID       string             `json:"assignee_user_id"`
	MissingSenderEmail   bool               `json:"missing_sender_email"`
	MissingAssigneeEmail bool               `json:"missing_assignee_email"`
	Status               string             `json:"status"`
	ExpiresAt            pgtype.Timestamptz `json:"expires_at"`
	CreatedAt            pgtype.Timestamptz `json:"created_at"`
	UpdatedAt            pgtype.Timestamptz `json:"updated_at"`
}
