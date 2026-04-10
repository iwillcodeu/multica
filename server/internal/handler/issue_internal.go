package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/logger"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// CreateIssueAsUserRequest is an internal-only request that allows the server to
// create an issue with a specific creator user (resolved by email).
type CreateIssueAsUserRequest struct {
	WorkspaceID   string  `json:"workspace_id"`
	CreatorEmail  string  `json:"creator_email"`
	Title         string  `json:"title"`
	Description   *string `json:"description"`
	ProjectID     string  `json:"project_id"`
	AssigneeEmail string  `json:"assignee_email"`
	Category      string  `json:"category"`
	Priority      string  `json:"priority"`
	DueDate       *string `json:"due_date"` // RFC3339
}

// CreateIssueAsUserCore creates an issue as a member creator and member assignee (trusted caller).
func (h *Handler) CreateIssueAsUserCore(ctx context.Context, req CreateIssueAsUserRequest) (db.Issue, error) {
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID == "" {
		return db.Issue{}, errInvalidArg("workspace_id is required")
	}
	creatorEmail := strings.ToLower(strings.TrimSpace(req.CreatorEmail))
	if creatorEmail == "" {
		return db.Issue{}, errInvalidArg("creator_email is required")
	}
	if strings.TrimSpace(req.Title) == "" {
		return db.Issue{}, errInvalidArg("title is required")
	}
	projectID := strings.TrimSpace(req.ProjectID)
	if projectID == "" {
		return db.Issue{}, errInvalidArg("project_id is required")
	}
	assigneeEmail := strings.ToLower(strings.TrimSpace(req.AssigneeEmail))
	if assigneeEmail == "" {
		return db.Issue{}, errInvalidArg("assignee_email is required")
	}

	creator, err := h.Queries.GetUserByEmail(ctx, creatorEmail)
	if err != nil {
		return db.Issue{}, errInvalidArg("creator not found")
	}
	assigneeUser, err := h.Queries.GetUserByEmail(ctx, assigneeEmail)
	if err != nil {
		return db.Issue{}, errInvalidArg("assignee not found")
	}

	creatorMember, err := h.Queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
		UserID:      creator.ID,
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		return db.Issue{}, errForbidden("creator is not a member of the workspace")
	}

	assigneeMember, err := h.Queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
		UserID:      assigneeUser.ID,
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		return db.Issue{}, errInvalidArg("assignee is not a member of the workspace")
	}

	projectUUID := parseUUID(projectID)
	if !projectUUID.Valid {
		return db.Issue{}, errInvalidArg("invalid project_id")
	}
	if _, err := h.Queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{
		ID:          projectUUID,
		WorkspaceID: parseUUID(workspaceID),
	}); err != nil {
		return db.Issue{}, errInvalidArg("project not found")
	}

	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = "task"
	} else {
		var ok bool
		category, ok = normalizeIssueCategory(category)
		if !ok {
			return db.Issue{}, errInvalidArg("invalid category, expected bug, feature, or task")
		}
	}

	priority := strings.TrimSpace(req.Priority)
	if priority == "" {
		priority = "none"
	}

	var dueDate pgtype.Timestamptz
	if req.DueDate != nil && strings.TrimSpace(*req.DueDate) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.DueDate))
		if err != nil {
			return db.Issue{}, errInvalidArg("invalid due_date format, expected RFC3339")
		}
		dueDate = pgtype.Timestamptz{Time: t, Valid: true}
	}

	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return db.Issue{}, err
	}
	defer tx.Rollback(ctx)
	qtx := h.Queries.WithTx(tx)

	issueNumber, err := qtx.IncrementIssueCounter(ctx, parseUUID(workspaceID))
	if err != nil {
		slog.Warn("increment issue counter failed", "error", err, "workspace_id", workspaceID)
		return db.Issue{}, err
	}

	issue, err := qtx.CreateIssue(ctx, db.CreateIssueParams{
		WorkspaceID:  parseUUID(workspaceID),
		ProjectID:    projectUUID,
		Title:        strings.TrimSpace(req.Title),
		Description:  ptrToText(req.Description),
		Status:       "backlog",
		Priority:     priority,
		Category:     category,
		AssigneeType: pgtype.Text{String: "member", Valid: true},
		AssigneeID:   assigneeMember.UserID,
		CreatorType:  "member",
		CreatorID:    creatorMember.UserID,
		Position:     0,
		DueDate:      dueDate,
		Number:       issueNumber,
	})
	if err != nil {
		slog.Warn("create issue failed", "error", err, "workspace_id", workspaceID)
		return db.Issue{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Issue{}, err
	}
	return issue, nil
}

type invalidArgError struct{ msg string }

func (e invalidArgError) Error() string { return e.msg }

func errInvalidArg(msg string) error { return invalidArgError{msg: msg} }

type forbiddenError struct{ msg string }

func (e forbiddenError) Error() string { return e.msg }

func errForbidden(msg string) error { return forbiddenError{msg: msg} }

// CreateIssueAsUser is an internal-only HTTP endpoint; protect it with RequireInternalSecret.
func (h *Handler) CreateIssueAsUser(w http.ResponseWriter, r *http.Request) {
	var req CreateIssueAsUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	issue, err := h.CreateIssueAsUserCore(r.Context(), req)
	if err != nil {
		switch err.(type) {
		case invalidArgError:
			writeError(w, http.StatusBadRequest, err.Error())
			return
		case forbiddenError:
			writeError(w, http.StatusForbidden, err.Error())
			return
		default:
			writeError(w, http.StatusInternalServerError, "failed to create issue")
			return
		}
	}

	workspaceID := strings.TrimSpace(req.WorkspaceID)
	creatorEmail := strings.ToLower(strings.TrimSpace(req.CreatorEmail))
	prefix := h.getIssuePrefix(r.Context(), issue.WorkspaceID)
	resp := issueToResponse(issue, prefix)
	slog.Info(
		"issue created (internal)",
		append(
			logger.RequestAttrs(r),
			"issue_id", uuidToString(issue.ID),
			"title", issue.Title,
			"workspace_id", workspaceID,
			"creator_email", creatorEmail,
			"assignee_email", strings.ToLower(strings.TrimSpace(req.AssigneeEmail)),
		)...,
	)
	h.publish(protocol.EventIssueCreated, workspaceID, "member", uuidToString(issue.CreatorID), map[string]any{"issue": resp})
	if issue.AssigneeType.Valid && issue.AssigneeID.Valid {
		if h.shouldEnqueueAgentTask(r.Context(), issue) {
			h.TaskService.EnqueueTaskForIssue(r.Context(), issue)
		}
	}

	writeJSON(w, http.StatusCreated, resp)
}
