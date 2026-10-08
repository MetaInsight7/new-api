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
		"failed to fetch codex usage",
		"获取用量信息失败，请稍后重试",
		false,
	)
}

func GetCodexChannelRateLimitResetCredits(c *gin.Context) {
	fetchCodexChannelWhamData(
		c,
		service.FetchCodexWhamRateLimitResetCredits,
		"failed to fetch codex reset credits",
		"获取重置次数详情失败，请稍后重试",
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
		"failed to reset codex usage",
		"无法确认上游是否已消费重置次数，请先刷新用量和重置次数，不要立即重复重置",
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
		common.SysError("failed to parse oauth key: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "解析凭证失败，请检查渠道配置"})
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

	client, err := service.GetHttpClientWithProxy(ch.GetSetting().Proxy)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	statusCode, body, err := fetch(ctx, client, ch.GetBaseURL(), accessToken, accountID)
	if err != nil {
		common.SysError(logPrefix + ": upstream request failed")
		c.JSON(http.StatusOK, gin.H{"success": false, "message": userMessage})
		return
	}

	if (statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden) && strings.TrimSpace(oauthKey.RefreshToken) != "" {
		refreshed, refreshedChannel, refreshErr := service.RefreshCodexChannelCredential(c.Request.Context(), ch.Id, service.CodexCredentialRefreshOptions{ResetCaches: true, ExpectedKey: ch.Key})
		if refreshErr != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Codex credential refresh or persistence failed", "upstream_status": statusCode})
			return
		}
		accessToken, accountID = refreshed.AccessToken, refreshed.AccountID
		client, err = service.GetHttpClientWithProxy(refreshedChannel.GetSetting().Proxy)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid Codex proxy configuration"})
			return
		}
		ctx2, cancel2 := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel2()
		statusCode, body, err = fetch(ctx2, client, refreshedChannel.GetBaseURL(), accessToken, accountID)
		if err != nil {
			common.SysError(logPrefix + " after refresh: upstream request failed")
			c.JSON(http.StatusOK, gin.H{"success": false, "message": userMessage})
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
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Upstream did not confirm any reset windows; refresh usage before trying again", "upstream_status": statusCode})
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
