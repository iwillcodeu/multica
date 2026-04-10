package dingtalk

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

// VerifyRobotCallbackSignature validates DingTalk "robot receive message" callbacks.
//
// Per DingTalk docs:
// - timestamp must be within 1 hour of current time.
// - sign = Base64(HMAC-SHA256(appSecret, timestamp+"\n"+appSecret))
func VerifyRobotCallbackSignature(timestampMS string, signature string, appSecret string) bool {
	if strings.TrimSpace(signature) == "" || strings.TrimSpace(appSecret) == "" {
		return false
	}
	ms, err := strconv.ParseInt(strings.TrimSpace(timestampMS), 10, 64)
	if err != nil || ms <= 0 {
		return false
	}
	ts := time.UnixMilli(ms)
	now := time.Now()
	diff := now.Sub(ts)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Hour {
		return false
	}

	stringToSign := timestampMS + "\n" + appSecret
	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write([]byte(stringToSign))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}
