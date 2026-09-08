package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestListIssuesDateFieldBothIsCreatedOrUpdatedUnion(t *testing.T) {
	projectID := dbfx.Project(t, fmt.Sprintf("date-both-%d", time.Now().UnixNano()))
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	old := start.Add(-8 * 24 * time.Hour)

	createdToday := dbfx.Issue(t, "created today", testutil.Cols{
		"project_id": projectID,
		"created_at": now,
		"updated_at": now,
	})
	updatedToday := dbfx.Issue(t, "updated today", testutil.Cols{
		"project_id": projectID,
		"created_at": old,
		"updated_at": now,
	})
	dbfx.Issue(t, "outside window", testutil.Cols{
		"project_id": projectID,
		"created_at": old,
		"updated_at": old,
	})

	list := func(field string) map[string]bool {
		t.Helper()
		query := url.Values{
			"project_id": {projectID},
			"limit":      {"50"},
			"date_field": {field},
			"date_start": {start.Format(time.RFC3339Nano)},
			"date_end":   {end.Format(time.RFC3339Nano)},
		}
		var resp struct {
			Issues []IssueResponse `json:"issues"`
		}
		testutil.Call(t, testHandler.ListIssues, newRequest(
			http.MethodGet,
			"/api/issues?"+query.Encode(),
			nil,
		)).Want(http.StatusOK).JSON(&resp)
		present := make(map[string]bool, len(resp.Issues))
		for _, issue := range resp.Issues {
			present[issue.ID] = true
		}
		return present
	}

	created := list("created_at")
	if !created[createdToday] || created[updatedToday] {
		t.Fatalf("created_at: got %v, want only created-today", created)
	}

	updated := list("updated_at")
	if !updated[createdToday] || !updated[updatedToday] {
		t.Fatalf("updated_at: got %v, want created-today and updated-today", updated)
	}

	both := list("both")
	if !both[createdToday] || !both[updatedToday] {
		t.Fatalf("both: got %v, want created-today and updated-today", both)
	}
	if len(both) != 2 {
		t.Fatalf("both: got %d issues, want 2", len(both))
	}
}

func TestListIssuesRejectsUnknownDateField(t *testing.T) {
	query := url.Values{
		"limit":      {"1"},
		"date_field": {"due_date"},
		"date_start": {time.Now().UTC().Format(time.RFC3339Nano)},
		"date_end":   {time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)},
	}
	testutil.Call(t, testHandler.ListIssues, newRequest(
		http.MethodGet,
		"/api/issues?"+query.Encode(),
		nil,
	)).Want(http.StatusBadRequest)
}
