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
)

func sendDingTalkText(ctx context.Context, webhookURL, secret, content string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	endpoint, err := url.Parse(strings.TrimSpace(webhookURL))
	if err != nil || endpoint.Host == "" || endpoint.User != nil || (endpoint.Scheme != "https" && endpoint.Scheme != "http") {
		return fmt.Errorf("DingTalk webhook URL is invalid")
	}
	if secret != "" {
		timestamp := fmt.Sprint(time.Now().UnixMilli())
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(timestamp + "\n" + secret))
		query := endpoint.Query()
		query.Set("timestamp", timestamp)
		query.Set("sign", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		endpoint.RawQuery = query.Encode()
	}
	if err := ValidateSSRFProtectedFetchURLContext(ctx, endpoint.String()); err != nil {
		return fmt.Errorf("DingTalk webhook rejected by fetch policy")
	}
	client := GetSSRFProtectedHTTPClient()
	if client == nil {
		return fmt.Errorf("protected HTTP client is not initialized")
	}
	payload, err := common.Marshal(map[string]any{"msgtype": "text", "text": map[string]string{"content": content}})
	if err != nil {
		return fmt.Errorf("cannot encode DingTalk message")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("cannot create DingTalk request")
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("DingTalk request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("DingTalk returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(body) > 65536 {
		return fmt.Errorf("DingTalk returned an unreadable or oversized response")
	}
	var result struct {
		Code *int `json:"errcode"`
	}
	if err := common.Unmarshal(body, &result); err != nil || result.Code == nil || *result.Code != 0 {
		return fmt.Errorf("DingTalk did not confirm delivery")
	}
	return nil
}
