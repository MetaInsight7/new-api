package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/codex"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GetCodexChannelUsage(c *gin.Context) {
	fetchCodexChannelWhamData(
		c,
		service.FetchCodexWhamUsage,
		common.LogText("failed to fetch codex usage"),
		"Failed to fetch usage. Please try again later",
		false,
	)
}

func GetCodexChannelRateLimitResetCredits(c *gin.Context) {
	fetchCodexChannelWhamData(
		c,
		service.FetchCodexWhamRateLimitResetCredits,
		common.LogText("failed to fetch codex reset credits"),
		"Failed to fetch reset credit details. Please try again later",
		false,
	)
}

func ResetCodexChannelUsage(c *gin.Context) {
	requestID := uuid.NewString()
	fetchCodexChannelWhamData(
		c,
		func(ctx context.Context, client *http.Client, baseURL, accessToken, accountID string) (int, []byte, error) {
			return service.ConsumeCodexWhamRateLimitResetCreditWithRequestID(ctx, client, baseURL, accessToken, accountID, requestID)
		},
		common.LogText("failed to reset codex usage"),
		"Unable to confirm whether a reset credit was consumed. Refresh usage and reset credits before trying again",
		true,
	)
}

type codexWhamFetchFunc func(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	accessToken string,
	accountID string,
) (statusCode int, body []byte, err error)

func fetchCodexChannelWhamData(
	c *gin.Context,
	fetch codexWhamFetchFunc,
	logPrefix string,
	userMessage string,
	consume bool,
) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	ch, err := model.GetChannelById(channelId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ch == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel not found"})
		return
	}
	if ch.Type != constant.ChannelTypeCodex {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel type is not Codex"})
		return
	}
	if ch.ChannelInfo.IsMultiKey {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "multi-key channel is not supported"})
		return
	}

	oauthKey, err := codex.ParseOAuthKey(strings.TrimSpace(ch.Key))
	if err != nil {
		common.SysError(common.LogText("failed to parse oauth key: %s", err.Error()))
		common.ApiErrorT(c, "Failed to parse credentials. Please check the channel configuration")
		return
	}
	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	accountID := strings.TrimSpace(oauthKey.AccountID)
	if accessToken == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "codex channel: access_token is required"})
		return
	}
	if accountID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "codex channel: account_id is required"})
		return
	}

	setting := ch.GetSetting()
	client, err := service.GetHttpClientWithProxySettings(setting.Proxy, setting)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	statusCode, body, err := fetch(ctx, client, ch.GetBaseURL(), accessToken, accountID)
	if err != nil {
		common.SysError(logPrefix + ": " + err.Error())
		common.ApiErrorT(c, userMessage)
		return
	}

	if (statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden) && strings.TrimSpace(oauthKey.RefreshToken) != "" {
		refreshCtx, refreshCancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer refreshCancel()
		refreshed, refreshedChannel, refreshErr := service.RefreshCodexChannelCredential(refreshCtx, ch.Id, service.CodexCredentialRefreshOptions{ResetCaches: true, ExpectedKey: ch.Key})
		if refreshErr != nil {
			common.SysError(common.LogText("failed to refresh Codex credential: %s", refreshErr.Error()))
			common.ApiErrorT(c, userMessage)
			return
		}
		accessToken, accountID = refreshed.AccessToken, refreshed.AccountID
		refreshedSetting := refreshedChannel.GetSetting()
		client, err = service.GetHttpClientWithProxySettings(refreshedSetting.Proxy, refreshedSetting)
		if err != nil {
			common.SysError(common.LogText("failed to create Codex HTTP client after refresh: %s", err.Error()))
			common.ApiErrorT(c, userMessage)
			return
		}
		ctx2, cancel2 := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel2()
		statusCode, body, err = fetch(ctx2, client, refreshedChannel.GetBaseURL(), accessToken, accountID)
		if err != nil {
			common.SysError(common.LogText("%s after refresh: %s", logPrefix, err.Error()))
			common.ApiErrorT(c, userMessage)
			return
		}
	}

	ok := statusCode >= 200 && statusCode < 300
	var payload map[string]any
	if ok && common.Unmarshal(body, &payload) != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": userMessage, "upstream_status": statusCode})
		return
	}
	if ok && consume {
		var outcome struct {
			WindowsReset *int `json:"windows_reset"`
		}
		if common.Unmarshal(body, &outcome) != nil || outcome.WindowsReset == nil || *outcome.WindowsReset <= 0 {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Upstream did not confirm any reset windows.", "upstream_status": statusCode})
			return
		}
	}
	if ok && !consume && payload != nil {
		payload["account_id"] = accountID
	}
	resp := gin.H{
		"success":         ok,
		"message":         "",
		"upstream_status": statusCode,
		"data":            payload,
	}
	if !ok {
		resp["message"] = fmt.Sprintf("upstream status: %d", statusCode)
	}
	c.JSON(http.StatusOK, resp)
}
