package controller

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/codex"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const codexChannelOAuthPurpose = "codex_channel_oauth"

type codexAuthorizationFlow struct {
	Identity       model.AuthSessionIdentity `json:"identity"`
	Verifier       string                    `json:"verifier"`
	ChannelID      int                       `json:"channel_id"`
	CredentialHash string                    `json:"credential_hash,omitempty"`
	Proxy          string                    `json:"proxy,omitempty"`
}

func StartCodexOAuth(c *gin.Context)              { startCodexAuthorization(c, false) }
func StartCodexOAuthForChannel(c *gin.Context)    { startCodexAuthorization(c, true) }
func CompleteCodexOAuth(c *gin.Context)           { completeCodexAuthorization(c, false) }
func CompleteCodexOAuthForChannel(c *gin.Context) { completeCodexAuthorization(c, true) }

func authorizeCodexCredentialManagement(tx *gorm.DB, identity model.AuthSessionIdentity) error {
	if err := model.ValidateAuthSessionWithTx(tx, identity); err != nil {
		return err
	}
	var user model.User
	if err := tx.Select("id", "role").First(&user, identity.UserID).Error; err != nil {
		return err
	}
	if !authz.Can(user.Id, user.Role, authz.ChannelSensitiveWrite) {
		return errors.New("Codex credential permission denied")
	}
	return nil
}

func startCodexAuthorization(c *gin.Context, existing bool) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok || model.DB.Transaction(func(tx *gorm.DB) error { return authorizeCodexCredentialManagement(tx, identity) }) != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Codex authorization requires a live session and credential permission"})
		return
	}
	var request struct {
		Proxy string `json:"proxy"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<10)
	if c.ShouldBindJSON(&request) != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid Codex authorization request"})
		return
	}
	payload := codexAuthorizationFlow{Identity: identity, Proxy: strings.TrimSpace(request.Proxy)}
	if existing {
		channelID, err := strconv.Atoi(c.Param("id"))
		if err != nil || channelID <= 0 {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid channel ID"})
			return
		}
		channel, err := model.GetChannelById(channelID, true)
		if err != nil || channel.Type != constant.ChannelTypeCodex || channel.ChannelInfo.IsMultiKey {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "A single-key Codex channel is required"})
			return
		}
		payload.ChannelID = channelID
		payload.Proxy = channel.GetSetting().Proxy
		payload.CredentialHash = common.GenerateHMACWithKey([]byte("codex-oauth-key:"+common.SessionSecret), channel.Key)
	}
	if _, err := service.GetHttpClientWithProxy(payload.Proxy); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid Codex OAuth proxy configuration"})
		return
	}
	verifier, err := service.NewCodexPKCEVerifier()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to initialize Codex authorization"})
		return
	}
	payload.Verifier = verifier
	encoded, err := common.Marshal(payload)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to initialize Codex authorization"})
		return
	}
	expiresAt := time.Now().Add(10 * time.Minute)
	state, _, err := model.CreateAuthFlow(model.AuthFlowCreate{Purpose: codexChannelOAuthPurpose, Provider: "codex", Intent: model.AuthFlowIntentBind, UserId: identity.UserID, SessionId: identity.SessionID, Payload: string(encoded), ExpiresAt: expiresAt})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to store Codex authorization"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"authorize_url": service.CodexOAuthAuthorizationURL(state, verifier), "expires_at": expiresAt.Unix()}})
}

func completeCodexAuthorization(c *gin.Context, existing bool) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Codex authorization requires a live session"})
		return
	}
	var request struct {
		Input string `json:"input"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	if c.ShouldBindJSON(&request) != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid Codex callback input"})
		return
	}
	code, state, err := parseCodexAuthorizationInput(request.Input)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	channelID := 0
	if existing {
		channelID, err = strconv.Atoi(c.Param("id"))
		if err != nil || channelID <= 0 {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid channel ID"})
			return
		}
	}
	var payload codexAuthorizationFlow
	_, err = model.ConsumeAuthFlowWithAction(state, model.AuthFlowMatch{Purpose: codexChannelOAuthPurpose, Provider: "codex", Intent: model.AuthFlowIntentBind, UserId: identity.UserID, SessionId: identity.SessionID}, func(tx *gorm.DB, flow *model.AuthFlow) error {
		if common.Unmarshal([]byte(flow.Payload), &payload) != nil || payload.Identity != identity || payload.ChannelID != channelID {
			return model.ErrAuthFlowInvalid
		}
		return authorizeCodexCredentialManagement(tx, identity)
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Codex authorization is expired, already used or does not match this session/channel"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	result, err := service.ExchangeCodexAuthorizationCode(ctx, code, payload.Verifier, payload.Proxy)
	if err != nil {
		model.RecordAuditLog(c, model.AuditLog{UserId: identity.UserID, ActorRole: c.GetInt("role"), Category: model.AuditCategorySecurity, Action: "channel.codex.oauth", Content: "Codex authorization exchange failed", Success: false})
		common.SetContextKey(c, constant.ContextKeyAuditLogged, true)
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	accountID, ok := service.ExtractCodexAccountIDFromJWT(result.AccessToken)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Codex authorization response has no account ID; restart authorization"})
		return
	}
	email, _ := service.ExtractEmailFromJWT(result.IDToken)
	if email == "" {
		email, _ = service.ExtractEmailFromJWT(result.AccessToken)
	}
	credential := codex.OAuthKey{IDToken: result.IDToken, AccessToken: result.AccessToken, RefreshToken: result.RefreshToken, AccountID: accountID, Email: email, Type: "codex", LastRefresh: time.Now().Format(time.RFC3339), Expired: result.ExpiresAt.Format(time.RFC3339)}
	encoded, err := common.Marshal(credential)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to encode Codex credential"})
		return
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := authorizeCodexCredentialManagement(tx, identity); err != nil {
			return err
		}
		if !existing {
			return nil
		}
		var channel model.Channel
		if err := tx.First(&channel, channelID).Error; err != nil {
			return err
		}
		hash := common.GenerateHMACWithKey([]byte("codex-oauth-key:"+common.SessionSecret), channel.Key)
		if channel.Type != constant.ChannelTypeCodex || channel.ChannelInfo.IsMultiKey || subtle.ConstantTimeCompare([]byte(hash), []byte(payload.CredentialHash)) != 1 {
			return errors.New("channel credential changed; restart authorization")
		}
		updated := tx.Model(&model.Channel{}).Where(map[string]any{"id": channelID, "type": constant.ChannelTypeCodex, "key": channel.Key}).Update("key", string(encoded))
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errors.New("channel credential changed; restart authorization")
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Codex credential could not be saved; session, permission or channel changed"})
		return
	}
	model.RecordAuditLog(c, model.AuditLog{UserId: identity.UserID, ActorRole: c.GetInt("role"), Category: model.AuditCategorySecurity, Action: "channel.codex.oauth", Content: "Codex authorization completed", Success: true, Other: model.AuditOther{Op: &model.AuditOperation{Action: "channel.codex.oauth", Params: model.AuditFields{"channel_id": channelID}}}})
	common.SetContextKey(c, constant.ContextKeyAuditLogged, true)
	data := gin.H{"account_id": accountID, "email": email, "expires_at": credential.Expired, "channel_id": channelID}
	if existing {
		model.InitChannelCache()
	} else {
		data["key"] = string(encoded)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

func parseCodexAuthorizationInput(input string) (string, string, error) {
	input = strings.TrimSpace(input)
	invalid := errors.New("Paste the complete localhost callback URL, code/state query or code#state")
	if input == "" || len(input) > 16<<10 {
		return "", "", invalid
	}
	var query string
	if strings.Contains(input, "://") {
		callback, err := url.Parse(input)
		if err != nil || callback.Scheme+"://"+callback.Host+callback.Path != service.CodexOAuthRedirectURI || callback.User != nil || callback.Fragment != "" {
			return "", "", invalid
		}
		query = callback.RawQuery
	} else if strings.Contains(input, "=") {
		query = strings.TrimPrefix(input, "?")
	} else {
		code, state, found := strings.Cut(input, "#")
		if !found || strings.TrimSpace(code) == "" || strings.TrimSpace(state) == "" || strings.Contains(state, "#") {
			return "", "", invalid
		}
		return strings.TrimSpace(code), strings.TrimSpace(state), nil
	}
	values, err := url.ParseQuery(query)
	if err != nil || len(values["code"]) != 1 || len(values["state"]) != 1 || strings.TrimSpace(values.Get("code")) == "" || strings.TrimSpace(values.Get("state")) == "" || values.Get("error") != "" {
		return "", "", invalid
	}
	return values.Get("code"), values.Get("state"), nil
}
