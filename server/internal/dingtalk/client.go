package dingtalk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Client calls DingTalk Open APIs used by the XiaoAn bot integration.
type Client struct {
	HTTP          *http.Client
	AppKey        string
	AppSecret     string
	tokenMu       sync.Mutex
	cachedToken   string
	tokenExpireAt time.Time
}

func NewClient(appKey, appSecret string) *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 20 * time.Second},
		AppKey:    strings.TrimSpace(appKey),
		AppSecret: strings.TrimSpace(appSecret),
	}
}

func (c *Client) GetAccessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.cachedToken != "" && time.Now().Before(c.tokenExpireAt.Add(-30*time.Second)) {
		return c.cachedToken, nil
	}
	if c.AppKey == "" || c.AppSecret == "" {
		return "", fmt.Errorf("dingtalk app key/secret not configured")
	}
	u := "https://oapi.dingtalk.com/gettoken?appkey=" + url.QueryEscape(c.AppKey) + "&appsecret=" + url.QueryEscape(c.AppSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		ErrCode   int    `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("gettoken decode: %w", err)
	}
	if out.ErrCode != 0 || out.AccessToken == "" {
		return "", fmt.Errorf("gettoken failed: %d %s", out.ErrCode, out.ErrMsg)
	}
	c.cachedToken = out.AccessToken
	if out.ExpiresIn > 0 {
		c.tokenExpireAt = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second)
	} else {
		c.tokenExpireAt = time.Now().Add(90 * time.Minute)
	}
	return c.cachedToken, nil
}

// GetUserEmail returns the enterprise user's email from topapi/v2/user/get.
func (c *Client) GetUserEmail(ctx context.Context, accessToken, userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", fmt.Errorf("empty dingtalk user id")
	}
	if email, ok := lookupEmailOverride(userID); ok {
		return email, nil
	}
	u := "https://oapi.dingtalk.com/topapi/v2/user/get?access_token=" + url.QueryEscape(accessToken)
	form := "userid=" + url.QueryEscape(userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		ErrCode int             `json:"errcode"`
		ErrMsg  string          `json:"errmsg"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("user/get decode: %w", err)
	}
	if out.ErrCode != 0 {
		return "", fmt.Errorf("user/get failed: %d %s", out.ErrCode, out.ErrMsg)
	}
	var res struct {
		Email    string `json:"email"`
		OrgEmail string `json:"org_email"`
	}
	if err := json.Unmarshal(out.Result, &res); err != nil {
		return "", fmt.Errorf("user/get result: %w", err)
	}
	email := strings.ToLower(strings.TrimSpace(res.Email))
	if email == "" {
		email = strings.ToLower(strings.TrimSpace(res.OrgEmail))
	}
	if email == "" {
		if email, ok := lookupEmailOverride(userID); ok {
			return email, nil
		}
		return "", fmt.Errorf("user has no email in DingTalk directory (userID=%s)", userID)
	}
	return email, nil
}

// lookupEmailOverride maps DingTalk user IDs (staffId/dingtalkId) to emails via env.
//
// Format:
//   DINGTALK_XIAOAN_USER_EMAIL_MAP="id1=email1,id2=email2"
func lookupEmailOverride(userID string) (string, bool) {
	raw := strings.TrimSpace(os.Getenv("DINGTALK_XIAOAN_USER_EMAIL_MAP"))
	if raw == "" {
		return "", false
	}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.ToLower(strings.TrimSpace(parts[1]))
		if k == "" || v == "" {
			continue
		}
		if k == userID {
			return v, true
		}
	}
	return "", false
}

// SendGroupSampleText sends a sampleText robot message to a group (OpenAPI v1.0).
// Requires robotCode from the developer console and openConversationId from the inbound event.
func (c *Client) SendGroupSampleText(ctx context.Context, accessToken, openConversationID, robotCode, content string) error {
	openConversationID = strings.TrimSpace(openConversationID)
	robotCode = strings.TrimSpace(robotCode)
	if openConversationID == "" || robotCode == "" {
		return fmt.Errorf("openConversationId and robotCode are required")
	}
	paramObj, _ := json.Marshal(map[string]string{"content": content})
	payload := map[string]any{
		"openConversationId": openConversationID,
		"robotCode":          robotCode,
		"msgKey":             "sampleText",
		"msgParam":           string(paramObj),
	}
	raw, _ := json.Marshal(payload)
	u := "https://api.dingtalk.com/v1.0/robot/groupMessages/send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(string(raw)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-dingtalk-access-token", accessToken)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &out)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("robot send http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out.Code != "" && out.Code != "0" && out.Code != "OK" {
		return fmt.Errorf("robot send api error: %s %s", out.Code, out.Message)
	}
	return nil
}
