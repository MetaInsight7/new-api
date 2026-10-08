package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func BuildDingTalkCodexModelGovernanceAlertContent(record *model.CodexModelGovernanceRecord) string {
	affected := model.DecodeCodexModelGovernanceChannelIDs(record.AffectedChannelIDs)
	disabled := model.CodexModelGovernanceDisabledChannelIDs(*record)
	content := fmt.Sprintf("Codex model governance alert\nModel: %s\nStatus: %s\nSource: %s\nMatched Rule: %s\nAffected Channels: %v\nDisabled Channels: %v\nReason: %s\nReview in the Codex model governance page.", record.ModelName, record.Status, record.Source, record.MatchedRule, affected, disabled, record.LastError)
	if len(disabled) < len(affected) {
		content += "\nLinked channels are still serving user requests."
	}
	return model.SanitizeCodexModelGovernanceEvidence(content)
}

func NotifyDingTalkCodexModelGovernance(ctx context.Context, record *model.CodexModelGovernanceRecord) (bool, error) {
	setting := operation_setting.GetCodexModelGovernanceSetting()
	if !setting.DingTalkEnabled || record == nil {
		return false, nil
	}
	endpoint, err := url.Parse(strings.TrimSpace(setting.DingTalkWebhookURL))
	if err != nil || endpoint.Host == "" {
		return false, fmt.Errorf("DingTalk webhook URL is invalid")
	}
	if setting.DingTalkSecret != "" {
		timestamp := fmt.Sprint(time.Now().UnixMilli())
		mac := hmac.New(sha256.New, []byte(setting.DingTalkSecret))
		_, _ = mac.Write([]byte(timestamp + "\n" + setting.DingTalkSecret))
		query := endpoint.Query()
		query.Set("timestamp", timestamp)
		query.Set("sign", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		endpoint.RawQuery = query.Encode()
	}
	if err := ValidateSSRFProtectedFetchURL(endpoint.String()); err != nil {
		return false, fmt.Errorf("DingTalk webhook rejected by fetch policy")
	}
	client := GetSSRFProtectedHTTPClient()
	if client == nil {
		return false, fmt.Errorf("protected HTTP client is not initialized")
	}
	scope := record.ModelName + "\x00" + record.AffectedChannelIDs + "\x00" + model.EncodeCodexModelGovernanceChannelIDsForDisplay(model.CodexModelGovernanceDisabledChannelIDs(*record))
	key := fmt.Sprintf("codex-governance:%x", sha256.Sum256([]byte(scope)))
	token, err := common.GenerateRandomCharsKey(32)
	if err != nil {
		return false, err
	}
	cooldown := time.Duration(max(1, setting.AlertCooldownMinutes)) * time.Minute
	reservation, allowed, err := model.ReserveCodexModelGovernanceAlertCooldown(key, cooldown, 30*time.Second, token)
	if err != nil || !allowed {
		return false, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = model.RollbackCodexModelGovernanceAlertCooldown(reservation)
		}
	}()
	payload, err := common.Marshal(map[string]any{"msgtype": "text", "text": map[string]string{"content": BuildDingTalkCodexModelGovernanceAlertContent(record)}})
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return false, fmt.Errorf("cannot create DingTalk request")
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("DingTalk request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return false, fmt.Errorf("DingTalk returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(body) > 65536 {
		return false, fmt.Errorf("DingTalk returned an unreadable or oversized response")
	}
	var result struct {
		Code *int `json:"errcode"`
	}
	if err := common.Unmarshal(body, &result); err != nil || result.Code == nil || *result.Code != 0 {
		return false, fmt.Errorf("DingTalk did not confirm delivery")
	}
	committed = true
	if err := model.CommitCodexModelGovernanceAlertCooldown(reservation); err != nil {
		return true, err
	}
	return true, nil
}
