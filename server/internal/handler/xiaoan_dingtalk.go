package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/dingtalk"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// XiaoanDingTalkPing handles GET /xiaoanmsg (DingTalk and load balancers often probe the URL with GET).
func (h *Handler) XiaoanDingTalkPing(w http.ResponseWriter, r *http.Request) {
	// Some platforms validate by checking for a literal "success" response body.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("success"))
}

// XiaoanDingTalkCallback handles POST /xiaoanmsg (DingTalk enterprise robot HTTP callback).
func (h *Handler) XiaoanDingTalkCallback(w http.ResponseWriter, r *http.Request) {
	xerr := func(status int, msg string, attrs ...any) {
		// Always log errors for this endpoint so we can debug DingTalk payload differences.
		if status >= 400 {
			base := []any{"status", status, "path", r.URL.Path, "content_type", r.Header.Get("Content-Type"), "body_len", r.ContentLength}
			base = append(base, attrs...)
			slog.Warn("xiaoan: request rejected: "+msg, base...)
		}
		writeError(w, status, msg)
	}

	if r.Method != http.MethodPost {
		xerr(http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		xerr(http.StatusBadRequest, "read body")
		return
	}

	// DingTalk console sometimes probes with an empty body or non-JSON.
	raw := strings.TrimSpace(string(body))
	if raw == "" {
		writeXiaoanAck(w)
		return
	}
	if strings.Contains(strings.ToLower(raw), "check_url") || strings.Contains(raw, "地址校验") {
		writeXiaoanAck(w)
		return
	}

	var root map[string]interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		xerr(http.StatusBadRequest, "invalid json", "body_prefix", truncateRunes(strings.TrimSpace(string(body)), 200))
		return
	}

	if _, hasEnc := root["encrypt"]; hasEnc {
		// P0: encrypted suite callbacks need AES + token; see DingTalk docs.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":false,"error":"encrypted_callback_not_supported_in_p0"}`))
		return
	}

	appSecret := strings.TrimSpace(os.Getenv("DINGTALK_XIAOAN_APP_SECRET"))
	skipVerify := strings.TrimSpace(os.Getenv("DINGTALK_XIAOAN_SKIP_VERIFY")) == "1"
	if !skipVerify {
		if appSecret == "" {
			xerr(http.StatusInternalServerError, "DINGTALK_XIAOAN_APP_SECRET not configured")
			return
		}
		ts := firstNonEmpty(r.URL.Query().Get("timestamp"), r.Header.Get("timestamp"))
		sig := firstNonEmpty(r.URL.Query().Get("sign"), r.URL.Query().Get("signature"), r.Header.Get("sign"), r.Header.Get("signature"))
		if !dingtalk.VerifyRobotCallbackSignature(ts, sig, appSecret) {
			xerr(http.StatusUnauthorized, "invalid signature", "has_ts", ts != "", "has_sig", sig != "")
			return
		}
	}

	payload := unwrapDingTalkPayload(root)
	// Console "publish" / URL checks send a signed POST that is not a real chat message (no msgId or check_url text).
	if isDingTalkHTTPCallbackURLProbe(root, payload) {
		writeXiaoanAck(w)
		return
	}

	msgID := firstNonEmpty(
		getString(payload, "msgId", "MsgId", "messageId", "MessageId"),
		getString(root, "msgId", "MsgId"),
	)
	if msgID == "" {
		xerr(http.StatusBadRequest, "missing msg id", "keys", keysPresent(payload), "root_keys", keysPresent(root))
		return
	}

	convID := firstNonEmpty(
		getString(payload, "conversationId", "openConversationId", "chatId", "OpenConversationId"),
		getString(root, "conversationId", "openConversationId"),
	)
	if convID == "" {
		xerr(http.StatusBadRequest, "missing conversation id", "msg_id", msgID, "keys", keysPresent(payload), "root_keys", keysPresent(root))
		return
	}

	ctx := r.Context()
	existing, err := h.Queries.GetDingtalkXiaoanDeliveryByMessageID(ctx, msgID)
	if err == nil {
		// Idempotent: DingTalk retries must not create duplicates.
		_ = existing
		writeXiaoanAck(w)
		return
	}
	if !isNotFound(err) {
		slog.Error("xiaoan: load delivery", "error", err)
		xerr(http.StatusInternalServerError, "delivery lookup failed")
		return
	}

	mapping, err := h.Queries.GetDingtalkXiaoanChatMappingByConversationID(ctx, convID)
	if err != nil {
		if isNotFound(err) {
			xerr(http.StatusBadRequest, "conversation not mapped to workspace (insert dingtalk_xiaoan_chat_mapping)", "conversation_id", convID, "msg_id", msgID)
			return
		}
		xerr(http.StatusInternalServerError, "mapping lookup failed")
		return
	}
	wsID := uuidToString(mapping.WorkspaceID)

	_, err = h.Queries.InsertDingtalkXiaoanDelivery(ctx, db.InsertDingtalkXiaoanDeliveryParams{
		DingtalkMessageID: msgID,
		ConversationID:    convID,
		WorkspaceID:       mapping.WorkspaceID,
		Status:            "received",
		ErrorMessage:      pgtype.Text{},
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeXiaoanAck(w)
			return
		}
		slog.Error("xiaoan: insert delivery", "error", err)
		xerr(http.StatusInternalServerError, "delivery insert failed")
		return
	}

	text := extractTextContent(payload)
	senderUID := firstNonEmpty(
		getString(payload, "senderStaffId", "senderId", "senderUserId", "userId", "staffId"),
		getString(root, "senderStaffId", "senderId"),
	)
	botIDSet := parseCommaSet(os.Getenv("DINGTALK_XIAOAN_BOT_USER_IDS"))
	if len(botIDSet) == 0 {
		// Help operators configure bot IDs: log possible IDs from this callback.
		atUsers := extractAtUsers(payload)
		atIDs := flattenAtUserIDs(atUsers)
		chatbotUserID := strings.TrimSpace(getString(payload, "chatbotUserId", "chatbot_user_id", "chatbotUserid"))
		slog.Warn(
			"xiaoan: DINGTALK_XIAOAN_BOT_USER_IDS missing",
			"msg_id", msgID,
			"conversation_id", convID,
			"sender_staff_id", senderUID,
			"chatbot_user_id", chatbotUserID,
			"at_user_ids", atIDs,
		)
		go h.xiaoanFailAndNotify(context.Background(), msgID, convID, "创建失败：服务端未配置 DINGTALK_XIAOAN_BOT_USER_IDS（钉钉用户 id，逗号分隔）\n"+xiaoanTemplateCN())
		writeXiaoanAck(w)
		return
	}
	atUsers := extractAtUsers(payload)
	if !mentionsBot(atUsers, botIDSet) {
		// Not addressed to the bot; acknowledge without creating.
		writeXiaoanAck(w)
		return
	}

	// Minimal multi-round flow: if the message is an email supplement, apply overrides and
	// retry the last pending create session for this conversation.
	if isXiaoanEmailSupplementText(text) {
		senderForAudit := senderUID
		go h.xiaoanHandleEmailSupplement(context.Background(), msgID, convID, senderForAudit, text)
		writeXiaoanAck(w)
		return
	}

	assigneeCandidates := filterOutBots(atUsers, botIDSet)
	if len(assigneeCandidates) != 1 {
		go h.xiaoanFailAndNotify(context.Background(), msgID, convID, "创建失败：请只 @ 1 位责任人（除小安外）\n"+xiaoanTemplateCN())
		writeXiaoanAck(w)
		return
	}
	assigneeUID := assigneeCandidates[0].UserLookupID()
	if assigneeUID == "" {
		go h.xiaoanFailAndNotify(context.Background(), msgID, convID, "创建失败：无法解析责任人 id（请 @ 企业成员，不要 @ 机器人/外部联系人）\n"+xiaoanTemplateCN())
		writeXiaoanAck(w)
		return
	}

	job := xiaoanProcessJob{
		h:              h,
		msgID:          msgID,
		convID:         convID,
		workspaceID:    wsID,
		text:           text,
		senderUserID:   senderUID,
		assigneeUserID: assigneeUID,
	}
	go job.run()

	writeXiaoanAck(w)
}

func writeXiaoanAck(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"success":true}`))
}

func extractDingTalkTextContent(maps ...map[string]interface{}) string {
	for _, m := range maps {
		if m == nil {
			continue
		}
		if v, ok := m["text"].(map[string]interface{}); ok {
			if s, ok := v["content"].(string); ok {
				return s
			}
		}
		if s, ok := m["content"].(string); ok {
			return s
		}
	}
	return ""
}

// isDingTalkHTTPCallbackURLProbe detects DingTalk developer-console URL verification (publish / save callback).
func isDingTalkHTTPCallbackURLProbe(root, payload map[string]interface{}) bool {
	text := strings.TrimSpace(extractDingTalkTextContent(root, payload))
	lower := strings.ToLower(text)
	if strings.Contains(lower, "check_url") || strings.Contains(text, "地址校验") {
		return true
	}
	msgID := firstNonEmpty(
		getString(payload, "msgId", "MsgId", "messageId", "MessageId"),
		getString(root, "msgId", "MsgId"),
	)
	convID := firstNonEmpty(
		getString(payload, "conversationId", "openConversationId", "chatId", "OpenConversationId"),
		getString(root, "conversationId", "openConversationId"),
	)
	return msgID == "" && convID == ""
}

func (h *Handler) xiaoanFailAndNotify(ctx context.Context, msgID, convID, msg string) {
	if _, err := h.Queries.UpdateDingtalkXiaoanDeliveryFailed(ctx, db.UpdateDingtalkXiaoanDeliveryFailedParams{
		DingtalkMessageID: msgID,
		ErrorMessage:      strToText(truncateRunes(msg, 2000)),
	}); err != nil {
		slog.Error("xiaoan: mark delivery failed", "error", err)
	}
	cli := dingtalk.NewClient(os.Getenv("DINGTALK_XIAOAN_APP_KEY"), os.Getenv("DINGTALK_XIAOAN_APP_SECRET"))
	token, err := cli.GetAccessToken(ctx)
	if err != nil {
		return
	}
	robotCode := strings.TrimSpace(os.Getenv("DINGTALK_XIAOAN_ROBOT_CODE"))
	if robotCode == "" {
		return
	}
	_ = cli.SendGroupSampleText(ctx, token, convID, robotCode, msg)
}

type xiaoanProcessJob struct {
	h              *Handler
	msgID          string
	convID         string
	workspaceID    string
	text           string
	senderUserID   string
	assigneeUserID string

	title       string
	projectName string
	desc        *string
}

func (j *xiaoanProcessJob) run() {
	ctx := context.Background()
	cli := dingtalk.NewClient(os.Getenv("DINGTALK_XIAOAN_APP_KEY"), os.Getenv("DINGTALK_XIAOAN_APP_SECRET"))
	token, err := cli.GetAccessToken(ctx)
	if err != nil {
		j.fail(ctx, "创建失败：无法获取钉钉 access_token（检查 appKey/appSecret）\n"+xiaoanTemplateCN())
		return
	}

	projectName := strings.TrimSpace(j.projectName)
	title := strings.TrimSpace(j.title)
	desc := j.desc
	if projectName == "" || title == "" {
		kv := parseKeyValuePairs(j.text)
		projectName = firstNonEmpty(kv["项目"], kv["project"], kv["Project"])
		if strings.TrimSpace(projectName) == "" {
			j.fail(ctx, "创建失败：缺少：项目\n"+xiaoanTemplateCN())
			return
		}

		title = firstNonEmpty(kv["任务"], kv["标题"], kv["title"], kv["name"], kv["Name"])
		if strings.TrimSpace(title) == "" {
			title = deriveTitleFromFreeText(j.text, projectName)
		}
		if strings.TrimSpace(title) == "" {
			j.fail(ctx, "创建失败：缺少：任务\n"+xiaoanTemplateCN())
			return
		}

		if d := firstNonEmpty(kv["描述"], kv["备注"], kv["description"]); d != "" {
			desc = &d
		}
		if desc != nil {
			footer := "\n\n---\n[钉钉小安] msgId=" + j.msgID + " conversationId=" + j.convID
			*desc = *desc + footer
		}
	}

	if strings.TrimSpace(projectName) == "" {
		j.fail(ctx, "创建失败：缺少：项目\n"+xiaoanTemplateCN())
		return
	}

	if strings.TrimSpace(title) == "" {
		j.fail(ctx, "创建失败：缺少：任务\n"+xiaoanTemplateCN())
		return
	}

	projID, perr := j.resolveProjectID(ctx, projectName)
	if perr != nil {
		j.fail(ctx, perr.Error()+"\n"+xiaoanTemplateCN())
		return
	}

	creatorEmail, err := j.getUserEmailWithOverride(ctx, cli, token, j.senderUserID)
	if err != nil {
		if isXiaoanMissingEmailErr(err) {
			j.deferForEmail(ctx, projectName, title, desc, true, false, "创建失败：无法获取发送者邮箱（"+err.Error()+"）\n"+xiaoanEmailSupplementTemplateCN(j.senderUserID, j.assigneeUserID))
			return
		}
		j.fail(ctx, "创建失败：无法获取发送者邮箱（"+err.Error()+"）\n"+xiaoanTemplateCN())
		return
	}
	assigneeEmail, err := j.getUserEmailWithOverride(ctx, cli, token, j.assigneeUserID)
	if err != nil {
		if isXiaoanMissingEmailErr(err) {
			j.deferForEmail(ctx, projectName, title, desc, false, true, "创建失败：无法获取责任人邮箱（"+err.Error()+"）\n"+xiaoanEmailSupplementTemplateCN(j.senderUserID, j.assigneeUserID))
			return
		}
		j.fail(ctx, "创建失败：无法获取责任人邮箱（"+err.Error()+"）\n"+xiaoanTemplateCN())
		return
	}

	issue, err := j.h.CreateIssueAsUserCore(ctx, CreateIssueAsUserRequest{
		WorkspaceID:   j.workspaceID,
		CreatorEmail:  creatorEmail,
		Title:         title,
		Description:   desc,
		ProjectID:     projID,
		AssigneeEmail: assigneeEmail,
	})
	if err != nil {
		msg := "创建失败：" + err.Error() + "\n" + xiaoanTemplateCN()
		switch err.(type) {
		case invalidArgError:
			msg = "创建失败：" + err.Error() + "\n" + xiaoanTemplateCN()
		case forbiddenError:
			msg = "创建失败：你不在该工作区，无法创建\n" + xiaoanTemplateCN()
		}
		j.fail(ctx, msg)
		return
	}

	prefix := j.h.getIssuePrefix(ctx, issue.WorkspaceID)
	resp := issueToResponse(issue, prefix)
	j.h.publish(protocol.EventIssueCreated, j.workspaceID, "member", uuidToString(issue.CreatorID), map[string]any{"issue": resp})
	if issue.AssigneeType.Valid && issue.AssigneeID.Valid {
		if j.h.shouldEnqueueAgentTask(ctx, issue) {
			j.h.TaskService.EnqueueTaskForIssue(ctx, issue)
		}
	}

	if _, err := j.h.Queries.UpdateDingtalkXiaoanDeliveryCreated(ctx, db.UpdateDingtalkXiaoanDeliveryCreatedParams{
		DingtalkMessageID: j.msgID,
		IssueID:           issue.ID,
	}); err != nil {
		slog.Error("xiaoan: mark delivery created", "error", err)
	}

	// Close pending session if it matches this initiator message.
	if pending, err := j.h.Queries.GetActiveDingtalkXiaoanPendingIssueCreateByConversationID(ctx, j.convID); err == nil {
		if strings.TrimSpace(pending.InitiatorMessageID) == strings.TrimSpace(j.msgID) {
			_, _ = j.h.Queries.MarkDingtalkXiaoanPendingIssueCreateCompleted(ctx, j.convID)
		}
	}

	base := strings.TrimRight(firstNonEmpty(os.Getenv("MULTICA_APP_URL"), os.Getenv("PUBLIC_APP_URL"), "http://localhost:3000"), "/")
	link := base + "/issues/" + uuidToString(issue.ID)
	msg := "已创建 Issue：" + issue.Title + "\n链接：" + link
	_ = j.notifyGroup(ctx, cli, token, msg)
}

func isXiaoanEmailSupplementText(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	if strings.Contains(t, "补充") {
		return true
	}
	return strings.Contains(t, "发送者邮箱=") || strings.Contains(t, "责任人邮箱=") || strings.Contains(t, "assignee_email=") || strings.Contains(t, "creator_email=")
}

func xiaoanEmailSupplementTemplateCN(senderUserID, assigneeUserID string) string {
	return "请补充邮箱后再 @ 小安：\n" +
		"- 发送者 userID=" + senderUserID + "\n" +
		"- 责任人 userID=" + assigneeUserID + "\n" +
		"回复示例：\n" +
		"@小安 补充 发送者邮箱=xxx@atuofuture.com; 责任人邮箱=yyy@atuofuture.com"
}

func (h *Handler) xiaoanHandleEmailSupplement(ctx context.Context, msgID, convID, updatedByUserID, text string) {
	pending, err := h.Queries.GetActiveDingtalkXiaoanPendingIssueCreateByConversationID(ctx, convID)
	if err != nil {
		if isNotFound(err) {
			h.xiaoanFailAndNotify(ctx, msgID, convID, "没有待补全的任务会话；请重新发送创建格式。\n"+xiaoanTemplateCN())
			return
		}
		slog.Error("xiaoan: load pending create", "error", err)
		return
	}

	kv := parseKeyValuePairs(text)
	senderEmail := firstNonEmpty(kv["发送者邮箱"], kv["creator_email"], kv["creatorEmail"])
	assigneeEmail := firstNonEmpty(kv["责任人邮箱"], kv["assignee_email"], kv["assigneeEmail"])
	if strings.TrimSpace(senderEmail) == "" && strings.TrimSpace(assigneeEmail) == "" {
		h.xiaoanFailAndNotify(ctx, msgID, convID, "未识别到邮箱补充字段；请按示例回复。\n"+xiaoanEmailSupplementTemplateCN(pending.SenderUserID, pending.AssigneeUserID))
		return
	}

	if s := strings.TrimSpace(senderEmail); s != "" {
		_, _ = h.Queries.UpsertDingtalkXiaoanUserEmailOverride(ctx, db.UpsertDingtalkXiaoanUserEmailOverrideParams{
			UserID:               pending.SenderUserID,
			Email:                strings.ToLower(s),
			SourceConversationID: strToText(convID),
			UpdatedByUserID:      strToText(updatedByUserID),
		})
	}
	if s := strings.TrimSpace(assigneeEmail); s != "" {
		_, _ = h.Queries.UpsertDingtalkXiaoanUserEmailOverride(ctx, db.UpsertDingtalkXiaoanUserEmailOverrideParams{
			UserID:               pending.AssigneeUserID,
			Email:                strings.ToLower(s),
			SourceConversationID: strToText(convID),
			UpdatedByUserID:      strToText(updatedByUserID),
		})
	}

	var desc *string
	if pending.Description.Valid {
		d := pending.Description.String
		desc = &d
	}

	job := xiaoanProcessJob{
		h:              h,
		msgID:          pending.InitiatorMessageID,
		convID:         convID,
		workspaceID:    uuidToString(pending.WorkspaceID),
		text:           "",
		senderUserID:   pending.SenderUserID,
		assigneeUserID: pending.AssigneeUserID,
		title:          pending.Title,
		projectName:    pending.ProjectName,
		desc:           desc,
	}
	job.run()
}

func isXiaoanMissingEmailErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "has no email") || strings.Contains(s, "no email")
}

func (j *xiaoanProcessJob) getUserEmailWithOverride(ctx context.Context, cli *dingtalk.Client, token, userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", errors.New("missing userID")
	}

	if ov, err := j.h.Queries.GetDingtalkXiaoanUserEmailOverride(ctx, userID); err == nil {
		if s := strings.ToLower(strings.TrimSpace(ov.Email)); s != "" {
			return s, nil
		}
	} else if !isNotFound(err) {
		slog.Warn("xiaoan: load email override failed", "error", err, "user_id", userID)
	}

	email, err := cli.GetUserEmail(ctx, token, userID)
	if err != nil {
		return "", err
	}
	norm := strings.ToLower(strings.TrimSpace(email))
	if norm != "" {
		// Persist successful resolution (DingTalk API or DINGTALK_XIAOAN_USER_EMAIL_MAP) so future
		// lookups use the database and we don't need to ask again or rely on .env.
		if _, err := j.h.Queries.UpsertDingtalkXiaoanUserEmailOverride(ctx, db.UpsertDingtalkXiaoanUserEmailOverrideParams{
			UserID:               userID,
			Email:                norm,
			SourceConversationID: strToText(j.convID),
			UpdatedByUserID:      strToText("resolved"),
		}); err != nil {
			slog.Warn("xiaoan: persist email binding failed", "error", err, "user_id", userID)
		}
	}
	return email, nil
}

func (j *xiaoanProcessJob) deferForEmail(
	ctx context.Context,
	projectName string,
	title string,
	desc *string,
	missingSender bool,
	missingAssignee bool,
	userFacingMsg string,
) {
	// Mark delivery as waiting for completion; do not treat as final failure.
	_, _ = j.h.Queries.UpdateDingtalkXiaoanDeliveryNeedsEmail(ctx, db.UpdateDingtalkXiaoanDeliveryNeedsEmailParams{
		DingtalkMessageID: j.msgID,
		ErrorMessage:      strToText(truncateRunes(userFacingMsg, 2000)),
	})

	exp := time.Now().Add(30 * time.Minute)
	_, _ = j.h.Queries.UpsertDingtalkXiaoanPendingIssueCreate(ctx, db.UpsertDingtalkXiaoanPendingIssueCreateParams{
		ConversationID:       j.convID,
		WorkspaceID:          parseUUID(j.workspaceID),
		InitiatorMessageID:   j.msgID,
		Title:                title,
		ProjectName:          projectName,
		Description:          ptrToText(desc),
		SenderUserID:         j.senderUserID,
		AssigneeUserID:       j.assigneeUserID,
		MissingSenderEmail:   missingSender,
		MissingAssigneeEmail: missingAssignee,
		ExpiresAt:            pgtype.Timestamptz{Time: exp, Valid: true},
	})

	cli := dingtalk.NewClient(os.Getenv("DINGTALK_XIAOAN_APP_KEY"), os.Getenv("DINGTALK_XIAOAN_APP_SECRET"))
	token, err := cli.GetAccessToken(ctx)
	if err != nil {
		return
	}
	_ = j.notifyGroup(ctx, cli, token, userFacingMsg)
}

func (j *xiaoanProcessJob) fail(ctx context.Context, msg string) {
	if _, err := j.h.Queries.UpdateDingtalkXiaoanDeliveryFailed(ctx, db.UpdateDingtalkXiaoanDeliveryFailedParams{
		DingtalkMessageID: j.msgID,
		ErrorMessage:      strToText(truncateRunes(msg, 2000)),
	}); err != nil {
		slog.Error("xiaoan: mark delivery failed", "error", err)
	}
	cli := dingtalk.NewClient(os.Getenv("DINGTALK_XIAOAN_APP_KEY"), os.Getenv("DINGTALK_XIAOAN_APP_SECRET"))
	token, err := cli.GetAccessToken(ctx)
	if err != nil {
		slog.Warn("xiaoan: fail notify token", "error", err)
		return
	}
	_ = j.notifyGroup(ctx, cli, token, msg)
}

func (j *xiaoanProcessJob) notifyGroup(ctx context.Context, cli *dingtalk.Client, token, text string) error {
	robotCode := strings.TrimSpace(os.Getenv("DINGTALK_XIAOAN_ROBOT_CODE"))
	if robotCode == "" {
		slog.Warn("xiaoan: DINGTALK_XIAOAN_ROBOT_CODE not set; skipping group reply")
		return nil
	}
	return cli.SendGroupSampleText(ctx, token, j.convID, robotCode, text)
}

func (j *xiaoanProcessJob) resolveProjectID(ctx context.Context, name string) (string, error) {
	wsUUID := parseUUID(j.workspaceID)
	list, err := j.h.Queries.ListProjectsByWorkspace(ctx, wsUUID)
	if err != nil {
		return "", errors.New("系统错误：无法列出项目")
	}
	want := strings.TrimSpace(name)
	var hits []db.Project
	for _, p := range list {
		if strings.EqualFold(strings.TrimSpace(p.Name), want) {
			hits = append(hits, p)
		}
	}
	if len(hits) == 0 {
		return "", errors.New("未找到项目：" + want)
	}
	if len(hits) > 1 {
		return "", errors.New("项目不唯一：" + want)
	}
	return uuidToString(hits[0].ID), nil
}

func xiaoanTemplateCN() string {
	return "请按以下格式一条消息重发：\n@小安 任务=<任务名>; 项目=<项目名>; 责任人=@<某人>\n（可选）描述=<补充说明>"
}

func unwrapDingTalkPayload(root map[string]interface{}) map[string]interface{} {
	if v, ok := root["event"].(map[string]interface{}); ok {
		return v
	}
	if v, ok := root["bizData"].(map[string]interface{}); ok {
		return v
	}
	return root
}

func getString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

func keysPresent(m map[string]interface{}) []string {
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func extractTextContent(m map[string]interface{}) string {
	if t, ok := m["text"].(map[string]interface{}); ok {
		if c, ok := t["content"].(string); ok {
			return c
		}
	}
	if c, ok := m["content"].(string); ok {
		return c
	}
	if b, ok := m["body"].(map[string]interface{}); ok {
		if c, ok := b["content"].(string); ok {
			return c
		}
	}
	return ""
}

func extractAtUserIDs(m map[string]interface{}) []string {
	// Backwards-compat wrapper; keep for existing call sites if any.
	return flattenAtUserIDs(extractAtUsers(m))
}

type atUserIDs struct {
	DingTalkID string
	StaffID    string
	UserID     string
}

func (u atUserIDs) AnyIDMatches(set map[string]bool) bool {
	if set[strings.TrimSpace(u.DingTalkID)] {
		return true
	}
	if set[strings.TrimSpace(u.StaffID)] {
		return true
	}
	if set[strings.TrimSpace(u.UserID)] {
		return true
	}
	return false
}

func (u atUserIDs) UserLookupID() string {
	// Prefer ids accepted by `topapi/v2/user/get`.
	if s := strings.TrimSpace(u.UserID); s != "" {
		return s
	}
	if s := strings.TrimSpace(u.StaffID); s != "" {
		return s
	}
	// `dingtalkId` for robots often starts with "$:" and is not accepted by `user/get`.
	if s := strings.TrimSpace(u.DingTalkID); s != "" && !strings.HasPrefix(s, "$:") {
		return s
	}
	return ""
}

func extractAtUsers(m map[string]interface{}) []atUserIDs {
	raw, ok := m["atUsers"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	var out []atUserIDs
	for _, it := range arr {
		mm, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, atUserIDs{
			DingTalkID: getString(mm, "dingtalkId", "DingtalkId"),
			StaffID:    getString(mm, "staffId", "StaffId"),
			UserID:     getString(mm, "userId", "UserId"),
		})
	}
	return out
}

func flattenAtUserIDs(users []atUserIDs) []string {
	var out []string
	for _, u := range users {
		if s := strings.TrimSpace(u.DingTalkID); s != "" {
			out = append(out, s)
		}
		if s := strings.TrimSpace(u.StaffID); s != "" {
			out = append(out, s)
		}
		if s := strings.TrimSpace(u.UserID); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func parseCommaSet(s string) map[string]bool {
	out := make(map[string]bool)
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out[p] = true
		}
	}
	return out
}

func mentionsBot(users []atUserIDs, bots map[string]bool) bool {
	for _, u := range users {
		if u.AnyIDMatches(bots) {
			return true
		}
	}
	return false
}

func filterOutBots(users []atUserIDs, bots map[string]bool) []atUserIDs {
	var out []atUserIDs
	for _, u := range users {
		if !u.AnyIDMatches(bots) {
			out = append(out, u)
		}
	}
	return out
}

func parseKeyValuePairs(s string) map[string]string {
	out := make(map[string]string)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ';' || r == '\n'
	})
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.IndexByte(part, '=')
		if idx <= 0 {
			continue
		}
		k := strings.TrimSpace(part[:idx])
		v := strings.TrimSpace(part[idx+1:])
		if k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}

func deriveTitleFromFreeText(full, projectName string) string {
	// Remove common key=value segments and use remainder as title (P0 heuristic).
	lines := strings.Split(strings.ReplaceAll(full, ";", "\n"), "\n")
	var kept []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "=") {
			k := strings.TrimSpace(line[:strings.IndexByte(line, '=')])
			if k == "项目" || k == "project" || k == "描述" || k == "备注" {
				continue
			}
		}
		kept = append(kept, line)
	}
	s := strings.TrimSpace(strings.Join(kept, " "))
	s = strings.ReplaceAll(s, "项目="+projectName, "")
	s = strings.TrimSpace(s)
	return s
}

func truncateRunes(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}
