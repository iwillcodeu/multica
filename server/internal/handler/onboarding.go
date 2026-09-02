package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/logger"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/middleware"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Upper bound on free-text fields. `cloudWaitlistReasonMaxLen` is a
// product cap ("we don't need an essay for a waitlist"); the body-size
// cap further down is defense in depth against arbitrary storage
// abuse via the JSON body.
const (
	cloudWaitlistReasonMaxLen = 500

	// PatchOnboarding body is a tiny JSON with at most a 3-question
	// questionnaire. 16 KiB is ~10x the realistic ceiling — it's the
	// minimum that keeps the door open for future fields without
	// letting a malicious user stuff the JSONB column.
	patchOnboardingBodyLimit = 16 * 1024
)

// completeOnboardingRequest carries the client's view of which exit the
// user took from the flow. Used purely as an analytics dimension — server
// state (onboarded_at) flips the same way regardless. Unknown / missing
// → OnboardingPathUnknown so legacy clients still complete cleanly, just
// without a funnel-ready label.
//
// `workspace_id` is retained for analytics enrichment. The handler itself
// does not create agents; current runtime-connected clients create their
// Mika onboarding chat before calling this endpoint. The explicit no-runtime
// path still seeds one setup guide from the frontend.
type completeOnboardingRequest struct {
	CompletionPath string `json:"completion_path,omitempty"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
}

var validCompletionPaths = map[string]struct{}{
	analytics.OnboardingPathFull:           {},
	analytics.OnboardingPathRuntimeSkipped: {},
	analytics.OnboardingPathCloudWaitlist:  {},
	analytics.OnboardingPathSkipExisting:   {},
	analytics.OnboardingPathInviteAccept:   {},
}

// CompleteOnboarding marks the authenticated user as having completed
// onboarding. Idempotent: the underlying query uses COALESCE so the
// original timestamp is preserved if called more than once.
//
// Emits `onboarding_completed` exactly once — the first call that
// actually flips `onboarded_at` from NULL. Subsequent calls are still
// 200 OK (for client-side retries) but skip the event so the funnel
// counts honest first-completion.
//
// Current clients have no in-handler agent-creation side effect. The
// runtime-connected flow creates Mika and starts its onboarding chat first,
// while the explicit no-runtime path may create a setup-guide issue after
// navigation. This handler itself does one thing: flip the field.
func (h *Handler) CompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	// Body is optional — an empty body is a legal legacy call.
	var req completeOnboardingRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	// Validate workspace_id if supplied; we don't write with it, but a
	// malformed value should fail fast rather than silently land in
	// PostHog as a junk dimension.
	if req.WorkspaceID != "" {
		wsUUID, ok := parseUUIDOrBadRequest(w, req.WorkspaceID, "workspace_id")
		if !ok {
			return
		}
		req.WorkspaceID = uuidToString(wsUUID)
	}

	before, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to complete onboarding")
		return
	}
	firstCompletion := !before.OnboardedAt.Valid

	user, err := h.Queries.MarkUserOnboarded(r.Context(), parseUUID(userID))
	if err != nil {
		slog.Warn("complete onboarding: mark user onboarded failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to complete onboarding")
		return
	}

	if firstCompletion {
		path := req.CompletionPath
		if _, ok := validCompletionPaths[path]; !ok {
			path = analytics.OnboardingPathUnknown
		}
		onboardedAt := ""
		if user.OnboardedAt.Valid {
			onboardedAt = user.OnboardedAt.Time.UTC().Format("2006-01-02T15:04:05Z07:00")
		}
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.OnboardingCompleted(
			userID,
			req.WorkspaceID,
			path,
			onboardedAt,
			user.CloudWaitlistEmail.Valid,
		))
	}

	payload, err := h.marshalUser(r.Context(), user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user flags")
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

type patchOnboardingRequest struct {
	Questionnaire *json.RawMessage `json:"questionnaire,omitempty"`
}

// questionnaireAnswers mirrors the frontend's `QuestionnaireAnswers`
// shape. `use_case` is multi-select (Step 3 allows picking several);
// `source` is single-select (primary acquisition channel) but kept
// as `stringOrSlice` for back-compat with v2 multi-select rows — the
// client now always commits a one-element array. `role` stays
// single-select.
//
// stringOrSlice also tolerates pre-array rows that wrote a bare
// string into the JSONB column — `json.Unmarshal` would otherwise
// fail on type mismatch when reading those back.
type stringOrSlice []string

func (s *stringOrSlice) UnmarshalJSON(data []byte) error {
	// Empty / null both decode to nil slice.
	if len(data) == 0 || string(data) == "null" {
		*s = nil
		return nil
	}
	// Try array first (current shape).
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*s = arr
		return nil
	}
	// Fall back to single string (pre-array shape from before this
	// column held a slice). Empty string means "unanswered" — keep nil.
	var single string
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}
	if single == "" {
		*s = nil
		return nil
	}
	*s = []string{single}
	return nil
}

type questionnaireAnswers struct {
	Source         stringOrSlice `json:"source"`
	SourceOther    string        `json:"source_other"`
	SourceSkipped  bool          `json:"source_skipped"`
	Role           string        `json:"role"`
	RoleOther      string        `json:"role_other"`
	RoleSkipped    bool          `json:"role_skipped"`
	UseCase        stringOrSlice `json:"use_case"`
	UseCaseOther   string        `json:"use_case_other"`
	UseCaseSkipped bool          `json:"use_case_skipped"`
	Version        int           `json:"version"`
}

func (q questionnaireAnswers) sourceResolved() bool {
	return len(q.Source) > 0 || q.SourceSkipped
}
func (q questionnaireAnswers) roleResolved() bool {
	return q.Role != "" || q.RoleSkipped
}
func (q questionnaireAnswers) useCaseResolved() bool {
	return len(q.UseCase) > 0 || q.UseCaseSkipped
}

// questionnaireSchemaVersion is the schema this handler understands.
// `complete()` and the funnel events are scoped to this version so a
// future v3 row can't be silently mis-counted against v2 semantics.
const questionnaireSchemaVersion = 2

// complete covers the IN-FLOW questionnaire only: role + use_case.
// Source moved out of the onboarding flow (MUL-5159) — it is collected
// later by the workspace backfill prompt, and its resolution is
// tracked by the separate `onboarding_source_submitted` emission in
// PatchOnboarding. Requiring source here would stall the funnel's
// "questionnaire submitted" step for days (or forever, for users who
// never see the backfill prompt).
func (q questionnaireAnswers) complete() bool {
	if q.Version != questionnaireSchemaVersion {
		return false
	}
	return q.roleResolved() && q.useCaseResolved()
}

// PatchOnboarding persists the user's questionnaire answers. The
// field is optional; an omitted questionnaire is preserved. Which
// step the user is on is deliberately not persisted — every
// onboarding entry starts at Welcome.
//
// Emits `onboarding_questionnaire_submitted` exactly once per user:
// the first PATCH that transitions role + use_case from "at least one
// slot empty" to "both resolved". Emits `onboarding_source_submitted`
// exactly once on the source slot's own unresolved → resolved
// transition, which normally happens later via the workspace backfill
// prompt. Revisions past those points don't re-emit — the funnel
// counts users, not edits.
func (h *Handler) PatchOnboarding(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	// Bound the body so the JSONB column can't be weaponized as bulk
	// storage — otherwise every subsequent `/api/me` read would have
	// to return the bloat.
	r.Body = http.MaxBytesReader(w, r.Body, patchOnboardingBodyLimit)
	var req patchOnboardingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Read prior answers so we can detect the NULL/partial → complete
	// transition after the update. An errored decode on the prior row
	// is treated as "incomplete" — worst case we emit once more than
	// we should, never twice for the same transition.
	var before questionnaireAnswers
	beforeRaw := []byte("{}")
	if beforeUser, err := h.Queries.GetUser(r.Context(), parseUUID(userID)); err == nil {
		beforeRaw = beforeUser.OnboardingQuestionnaire
		_ = json.Unmarshal(beforeRaw, &before)
	}
	// firstTouch is true when the user has never written any
	// onboarding state on the server before this PATCH. Used to fire
	// onboarding_started exactly once per user from the server side.
	firstTouch := len(beforeRaw) == 0 || string(beforeRaw) == "null" || string(beforeRaw) == "{}"

	params := db.PatchUserOnboardingParams{ID: parseUUID(userID)}
	if req.Questionnaire != nil {
		params.Questionnaire = []byte(*req.Questionnaire)
	}
	user, err := h.Queries.PatchUserOnboarding(r.Context(), params)
	if err != nil {
		slog.Warn("patch onboarding failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to update onboarding")
		return
	}

	// Server-side onboarding_started: fire on the first PATCH that
	// actually carries a questionnaire payload. The frontend also
	// emits its own onboarding_started on page open; the two together
	// let Grafana cross-check the funnel against PostHog.
	if firstTouch && req.Questionnaire != nil && len(*req.Questionnaire) > 0 && string(*req.Questionnaire) != "{}" {
		platform, _, _ := middleware.ClientMetadataFromContext(r.Context())
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.OnboardingStarted(userID, platform))
	}

	var after questionnaireAnswers
	_ = json.Unmarshal(user.OnboardingQuestionnaire, &after)
	if after.complete() && !before.complete() {
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.OnboardingQuestionnaireSubmitted(
			userID,
			[]string(after.Source),
			after.Role,
			[]string(after.UseCase),
			after.SourceSkipped,
			after.RoleSkipped,
			after.UseCaseSkipped,
			after.SourceOther != "",
			after.RoleOther != "",
			after.UseCaseOther != "",
		))
	}

	// Source resolves on its own timeline — typically days after the
	// in-flow questionnaire, via the workspace backfill prompt (it can
	// no longer resolve in-flow). Emit on the unresolved → resolved
	// transition so the backfill prompt's answer/decline rate shows up
	// in Grafana; the transition check keeps the emission
	// once-per-user, mirroring the questionnaire event above.
	if after.Version == questionnaireSchemaVersion &&
		after.sourceResolved() && !before.sourceResolved() {
		obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.OnboardingSourceSubmitted(
			userID,
			[]string(after.Source),
			after.SourceSkipped,
			after.SourceOther != "",
		))
	}

	payload, err := h.marshalUser(r.Context(), user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user flags")
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

type joinCloudWaitlistRequest struct {
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

// JoinCloudWaitlist records a user's interest in cloud runtimes.
// Pure side effect — does NOT complete onboarding. The user still
// has to pick a real Step 3 path (CLI with a detected runtime) or
// Skip to move on. Repeating the call overwrites email + reason.
func (h *Handler) JoinCloudWaitlist(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var req joinCloudWaitlistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// RFC 5321 caps email at 254 chars; the column is VARCHAR(254) and
	// the format check below rejects anything net/mail can't parse.
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	if len(email) > 254 {
		writeError(w, http.StatusBadRequest, "email is too long")
		return
	}
	if _, err := mail.ParseAddress(email); err != nil {
		writeError(w, http.StatusBadRequest, "email is invalid")
		return
	}

	reason := strings.TrimSpace(req.Reason)
	if len(reason) > cloudWaitlistReasonMaxLen {
		writeError(w, http.StatusBadRequest, "reason is too long")
		return
	}

	reasonParam := pgtype.Text{}
	if reason != "" {
		reasonParam = pgtype.Text{String: reason, Valid: true}
	}

	user, err := h.Queries.JoinCloudWaitlist(r.Context(), db.JoinCloudWaitlistParams{
		ID:                  parseUUID(userID),
		CloudWaitlistEmail:  pgtype.Text{String: email, Valid: true},
		CloudWaitlistReason: reasonParam,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to join waitlist")
		return
	}

	obsmetrics.RecordEvent(h.Analytics, h.Metrics, analytics.CloudWaitlistJoined(userID, reason != ""))

	payload, err := h.marshalUser(r.Context(), user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user flags")
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// -----------------------------------------------------------------------------
// Starter content (post-onboarding opt-in)
// -----------------------------------------------------------------------------
//
// Users land in their workspace with starter_content_state=NULL and see
// a one-time dialog offering to seed example content. Two terminal
// transitions:
//
//   ImportStarterContent  NULL -> 'imported'  (also creates project, welcome
//                                              issue if agent-based, sub-issues,
//                                              pins — all in one transaction)
//   DismissStarterContent NULL -> 'dismissed'
//
// Why state-first, then seeding inside the same transaction:
//   - starter_content_state is the "have we asked / done this" bit, so it
//     must be set exactly once per user
//   - if we set state AFTER creation, a mid-request crash leaves duplicates
//     on retry (the original "Not idempotent" bug)
//   - if we set state BEFORE creation, a mid-request crash leaves the user
//     with 'imported' + no content
//   - inside a transaction, both commit together or neither does — and the
//     starting state check (must be NULL) guarantees the claim is atomic
//
// Content generation lives in TypeScript (the markdown templates are large
// and depend on the Q1–Q3 answers); the client POSTs the fully-rendered
// payload here, and the server's job is to (1) gate on state, (2) do the
// batch insert transactionally, (3) record the transition.

type importIssueSpec struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	// AssignToSelf: true for sub-issues (assigned to the current
	// user as a member). Server uses `user_id` per the app-wide
	// convention in AssigneePicker / resolveActor.
	AssignToSelf bool `json:"assign_to_self"`
}

// welcomeIssueTemplate is a PRE-rendered welcome issue — title +
// description + priority. There is no `agent_id` field on purpose:
// the server picks the target agent itself from ListAgents inside
// the transaction, so a stale or compromised client can't assign
// the welcome issue to an arbitrary agent.
type welcomeIssueTemplate struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	// Priority optional; defaults to "high" when empty.
	Priority string `json:"priority"`
}

type importStarterContentRequest struct {
	WorkspaceID string `json:"workspace_id"`

	Project struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	} `json:"project"`

	// Welcome issue template — rendered regardless of branch. The
	// server creates it only when at least one agent exists in the
	// workspace; otherwise it's ignored.
	WelcomeIssueTemplate welcomeIssueTemplate `json:"welcome_issue_template"`

	// Both branches of sub-issues. The server picks which array to
	// seed based on whether the workspace has any agents at the
	// moment of the call — the client no longer decides. Sending
	// both is ~15 KB extra payload, which stays well under the
	// 64 KB MaxBytesReader cap above.
	AgentGuidedSubIssues []importIssueSpec `json:"agent_guided_sub_issues"`
	SelfServeSubIssues   []importIssueSpec `json:"self_serve_sub_issues"`
}

type importStarterContentResponse struct {
	User           UserResponse `json:"user"`
	ProjectID      string       `json:"project_id"`
	WelcomeIssueID *string      `json:"welcome_issue_id"`
}

// ImportStarterContent creates the Getting Started project, optional
// welcome issue, sub-issues, and pins — all inside a single transaction
// gated by the atomic NULL -> 'imported' state transition. Idempotent
// at the state level: any second call returns 409 with the already-set
// state, no duplicate content created.
func (h *Handler) ImportStarterContent(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, importStarterContentBodyLimit)
	var req importStarterContentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	// Reject malformed UUIDs up front and reuse the parsed value for every
	// write below so a garbage workspace_id never reaches CreateProject /
	// CreateIssue.
	wsUUID, ok := parseUUIDOrBadRequest(w, req.WorkspaceID, "workspace_id")
	if !ok {
		return
	}
	req.WorkspaceID = uuidToString(wsUUID)
	if req.Project.Title == "" {
		writeError(w, http.StatusBadRequest, "project.title is required")
		return
	}

	// Start the transaction early — the state claim lives inside it so
	// concurrent imports from another tab can't both pass the check.
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	// Claim step: user must be NULL (never asked) to proceed. A value
	// of 'imported' / 'dismissed' / 'skipped_legacy' all short-circuit
	// with 409 Conflict — the caller should close the dialog and
	// refresh the user to pick up the already-final state.
	user, err := qtx.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	if user.StarterContentState.Valid {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "starter content already decided",
			"state": user.StarterContentState.String,
		})
		return
	}

	// Membership check: user must belong to the target workspace.
	// `actorID` below is `parseUUID(userID)` — stored as `creator_id`
	// and `assignee_id` for `type="member"` to match the app-wide
	// convention (AssigneePicker + resolveActor). Storing `member.id`
	// would cause `useActorName.getMemberName` to resolve to "Unknown"
	// since members are looked up by `user_id`.
	if _, err := qtx.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{
		UserID:      parseUUID(userID),
		WorkspaceID: wsUUID,
	}); err != nil {
		writeError(w, http.StatusForbidden, "not a member of this workspace")
		return
	}
	actorID := parseUUID(userID)

	// --- Branch decision (server-authoritative) ---
	// Ask the DB — not the client — whether there's an agent in this
	// workspace. `ListAgents` orders by created_at ASC, so "agents[0]"
	// is deterministically the earliest-created agent. This replaces
	// the old client-supplied `welcome_issue.agent_id` trust chain.
	agents, err := qtx.ListAgents(r.Context(), wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agents")
		return
	}
	hasAgent := len(agents) > 0
	var welcomeAgentID pgtype.UUID
	if hasAgent {
		welcomeAgentID = agents[0].ID
	}
	subSpecs := req.SelfServeSubIssues
	if hasAgent {
		subSpecs = req.AgentGuidedSubIssues
	}

	// --- Create project ---
	project, err := qtx.CreateProject(r.Context(), db.CreateProjectParams{
		WorkspaceID: wsUUID,
		Title:       req.Project.Title,
		Description: strOrNullText(req.Project.Description),
		Icon:        strOrNullText(req.Project.Icon),
		Status:      "planned",
		Priority:    "none",
	})
	if err != nil {
		slog.Warn("import starter content: create project failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	// --- Create welcome issue (only when an agent exists) ---
	var welcomeIssueID *string
	var welcomeIssueForEvent *db.Issue
	if hasAgent && req.WelcomeIssueTemplate.Title != "" {
		welcomeNumber, err := qtx.IncrementIssueCounter(r.Context(), wsUUID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to allocate issue number")
			return
		}
		priority := req.WelcomeIssueTemplate.Priority
		if priority == "" {
			priority = "high"
		}
		welcome, err := qtx.CreateIssue(r.Context(), db.CreateIssueParams{
			WorkspaceID:  wsUUID,
			Title:        req.WelcomeIssueTemplate.Title,
			Description:  strOrNullText(req.WelcomeIssueTemplate.Description),
			Status:       "todo",
			Priority:     priority,
			Category:     "task",
			AssigneeType: pgtype.Text{String: "agent", Valid: true},
			AssigneeID:   welcomeAgentID,
			CreatorType:  "member",
			CreatorID:    actorID,
			Number:       welcomeNumber,
		})
		if err != nil {
			slog.Warn("import starter content: create welcome issue failed", append(logger.RequestAttrs(r), "error", err)...)
			writeError(w, http.StatusInternalServerError, "failed to create welcome issue")
			return
		}
		id := uuidToString(welcome.ID)
		welcomeIssueID = &id
		copy := welcome
		welcomeIssueForEvent = &copy
	}

	// --- Create sub-issues (branch picked above) ---
	subIssuesCreated := make([]db.Issue, 0, len(subSpecs))
	for _, sub := range subSpecs {
		if sub.Title == "" {
			continue
		}
		number, err := qtx.IncrementIssueCounter(r.Context(), wsUUID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to allocate issue number")
			return
		}
		var assigneeType pgtype.Text
		var assigneeID pgtype.UUID
		if sub.AssignToSelf {
			assigneeType = pgtype.Text{String: "member", Valid: true}
			assigneeID = actorID
		}
		status := sub.Status
		if status == "" {
			status = "backlog"
		}
		priority := sub.Priority
		if priority == "" {
			priority = "none"
		}
		issue, err := qtx.CreateIssue(r.Context(), db.CreateIssueParams{
			WorkspaceID:  wsUUID,
			Title:        sub.Title,
			Description:  strOrNullText(sub.Description),
			Status:       status,
			Priority:     priority,
			Category:     "task",
			AssigneeType: assigneeType,
			AssigneeID:   assigneeID,
			CreatorType:  "member",
			CreatorID:    actorID,
			Number:       number,
			ProjectID:    project.ID,
		})
		if err != nil {
			slog.Warn("import starter content: create sub-issue failed", append(logger.RequestAttrs(r), "error", err, "title", sub.Title)...)
			writeError(w, http.StatusInternalServerError, "failed to create sub-issues")
			return
		}
		subIssuesCreated = append(subIssuesCreated, issue)
	}

	// --- Pin project (and welcome issue if present) ---
	// Non-fatal: a pin failure shouldn't prevent the onboarding bundle
	// from landing. We warn and move on. Pointers to the created rows
	// are kept around for post-commit `pin:created` fan-out so the
	// sidebar refreshes without a manual reload.
	pinnedProjectPos := float64(1)
	var pinProjectForEvent *db.PinnedItem
	pinProject, err := qtx.CreatePinnedItem(r.Context(), db.CreatePinnedItemParams{
		WorkspaceID: wsUUID,
		UserID:      parseUUID(userID),
		ItemType:    "project",
		ItemID:      project.ID,
		Position:    pinnedProjectPos,
	})
	if err != nil {
		slog.Warn("import starter content: pin project failed", append(logger.RequestAttrs(r), "error", err)...)
	} else {
		pinProjectForEvent = &pinProject
	}
	var pinWelcomeIssueForEvent *db.PinnedItem
	if welcomeIssueForEvent != nil {
		pinWelcome, err := qtx.CreatePinnedItem(r.Context(), db.CreatePinnedItemParams{
			WorkspaceID: wsUUID,
			UserID:      parseUUID(userID),
			ItemType:    "issue",
			ItemID:      welcomeIssueForEvent.ID,
			Position:    pinnedProjectPos + 1,
		})
		if err != nil {
			slog.Warn("import starter content: pin welcome issue failed", append(logger.RequestAttrs(r), "error", err)...)
		} else {
			pinWelcomeIssueForEvent = &pinWelcome
		}
	}

	// --- Flip state ---
	updatedUser, err := qtx.SetStarterContentState(r.Context(), db.SetStarterContentStateParams{
		ID:                  parseUUID(userID),
		StarterContentState: pgtype.Text{String: "imported", Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record starter content state")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit starter content")
		return
	}

	// --- Post-commit: realtime events + agent task enqueue ---
	// Realtime fan-out happens here (not inside the tx) because the DB
	// commit must land first — otherwise subscribers could receive an
	// event for state that's about to be rolled back.
	projectResp := projectToResponse(project)
	h.publish(protocol.EventProjectCreated, req.WorkspaceID, "member", userID, map[string]any{"project": projectResp})

	workspacePrefix := h.getIssuePrefix(r.Context(), wsUUID)
	if welcomeIssueForEvent != nil {
		welcomeResp := issueToResponse(*welcomeIssueForEvent, workspacePrefix)
		h.publish(protocol.EventIssueCreated, req.WorkspaceID, "member", userID, map[string]any{"issue": welcomeResp})
		if h.shouldEnqueueAgentTask(r.Context(), *welcomeIssueForEvent) {
			h.TaskService.EnqueueTaskForIssue(r.Context(), *welcomeIssueForEvent)
		}
	}
	for _, sub := range subIssuesCreated {
		subResp := issueToResponse(sub, workspacePrefix)
		h.publish(protocol.EventIssueCreated, req.WorkspaceID, "member", userID, map[string]any{"issue": subResp})
	}
	// Pin events. Without these, the sidebar's `pinListOptions` query
	// stays cached on the pre-import snapshot — only a hard refresh
	// surfaces the new pins. Same payload shape as `POST /pins`.
	if pinProjectForEvent != nil {
		h.publish(protocol.EventPinCreated, req.WorkspaceID, "member", userID, map[string]any{"pin": pinnedItemToResponse(*pinProjectForEvent)})
	}
	if pinWelcomeIssueForEvent != nil {
		h.publish(protocol.EventPinCreated, req.WorkspaceID, "member", userID, map[string]any{"pin": pinnedItemToResponse(*pinWelcomeIssueForEvent)})
	}

	starterBranch := analytics.StarterContentBranchSelfServe
	if hasAgent {
		starterBranch = analytics.StarterContentBranchAgentGuided
	}
	h.Analytics.Capture(analytics.StarterContentDecided(userID, req.WorkspaceID, "imported", starterBranch))

	ur, err := h.marshalUser(r.Context(), updatedUser)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user flags")
		return
	}
	writeJSON(w, http.StatusOK, importStarterContentResponse{
		User:           ur,
		ProjectID:      uuidToString(project.ID),
		WelcomeIssueID: welcomeIssueID,
	})
}

type dismissStarterContentRequest struct {
	// WorkspaceID is optional but strongly preferred — when present the
	// server derives the starter branch (agent_guided / self_serve) by
	// looking at the workspace's current agent list, so analytics can
	// split dismiss rate by branch the same way import is split.
	// Without it, branch defaults to self_serve (the zero-agent case).
	WorkspaceID string `json:"workspace_id,omitempty"`
}

// DismissStarterContent records the user's decision to skip starter
// content. Like Import, this is a NULL -> terminal transition; a
// second call returns 409 with the current state.
//
// Emits `starter_content_decided` with `decision=dismissed`. The
// `branch` property mirrors what ImportStarterContent would have
// written for the same workspace, so the two-sided funnel (import vs
// dismiss by branch) stays directly comparable.
func (h *Handler) DismissStarterContent(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	// Body is optional for backward-compat with callers that pre-date
	// the workspace-id addition. An empty body is a legal dismiss.
	var req dismissStarterContentRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	user, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	if user.StarterContentState.Valid {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "starter content already decided",
			"state": user.StarterContentState.String,
		})
		return
	}

	// Resolve branch before the update so the analytics event mirrors
	// the import-side logic exactly. An unresolvable workspace (malformed
	// UUID, user not a member, or empty body) falls back to self_serve —
	// the conservative default that matches what Import would emit when
	// ListAgents returns empty.
	branch := analytics.StarterContentBranchSelfServe
	if req.WorkspaceID != "" {
		if wsUUID, err := util.ParseUUID(req.WorkspaceID); err == nil {
			req.WorkspaceID = uuidToString(wsUUID)
			if _, err := h.Queries.GetMemberByUserAndWorkspace(r.Context(), db.GetMemberByUserAndWorkspaceParams{
				UserID:      parseUUID(userID),
				WorkspaceID: wsUUID,
			}); err == nil {
				agents, err := h.Queries.ListAgents(r.Context(), wsUUID)
				if err == nil && len(agents) > 0 {
					branch = analytics.StarterContentBranchAgentGuided
				}
			}
		}
	}

	updated, err := h.Queries.SetStarterContentState(r.Context(), db.SetStarterContentStateParams{
		ID:                  parseUUID(userID),
		StarterContentState: pgtype.Text{String: "dismissed", Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record dismiss")
		return
	}

	h.Analytics.Capture(analytics.StarterContentDecided(userID, req.WorkspaceID, "dismissed", branch))

	payload, err := h.marshalUser(r.Context(), updated)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user flags")
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// strOrNullText converts an empty-meaning-absent string into a
// nullable pgtype.Text. Empty -> SQL NULL; non-empty -> Valid.
func strOrNullText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
