package codex

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
}

func codexClientWantsStream(c *gin.Context, info *relaycommon.RelayInfo) bool {
	if c != nil {
		if _, ok := common.GetContextKey(c, appconstant.ContextKeyIsStream); ok {
			return common.GetContextKeyBool(c, appconstant.ContextKeyIsStream)
		}
	}
	return info != nil && info.IsStream
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("codex channel: endpoint not supported")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("codex channel: /v1/messages endpoint not supported")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("codex channel: endpoint not supported")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if info == nil {
		return nil, types.NewErrorWithStatusCode(errors.New("codex channel: relay info is required"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if !IsCodexImageModel(request.Model) {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("codex channel: image endpoint requires a gpt-image-* model, got %q", request.Model), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if err := ValidateCodexImageRequest(request); err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	action := "generate"
	var inputImages []string
	var mask string
	if info.RelayMode == relayconstant.RelayModeImagesEdits {
		action = "edit"
		var err error
		if isCodexJSONRequest(c) {
			inputImages, mask, err = readCodexJSONEditImages(request)
		} else {
			inputImages, mask, err = readCodexEditImages(c)
		}
		if err != nil {
			return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
	}

	// Codex image generation only returns the image result through Responses SSE.
	info.IsStream = true
	body := buildCodexImageBody(request, resolveImageCarrierModel(info), action, inputImages, mask)
	setFingerprintIDs(c, nil)
	ids, err := resolveFingerprintIDsForRequest(info, clientSessionID(c), time.Now())
	if err != nil {
		return nil, err
	}
	setFingerprintIDs(c, ids)
	if ids != nil && !applyFingerprintBody(body, ids) {
		return nil, errors.New("codex channel: sanitize fingerprint metadata")
	}
	return body, nil
}

func isCodexJSONRequest(c *gin.Context) bool {
	return c == nil || c.Request == nil || strings.HasPrefix(strings.ToLower(c.Request.Header.Get("Content-Type")), "application/json")
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	responsesRequest, err := service.ChatCompletionsRequestToResponsesRequest(request)
	if err != nil {
		return nil, err
	}
	stream := true
	responsesRequest.Stream = &stream
	return a.ConvertOpenAIResponsesRequest(c, info, *responsesRequest)
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("codex channel: /v1/rerank endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("codex channel: /v1/embeddings endpoint not supported")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	isCompact := info != nil && info.RelayMode == relayconstant.RelayModeResponsesCompact
	setFingerprintIDs(c, nil)

	if info != nil && info.ChannelSetting.SystemPrompt != "" {
		systemPrompt := info.ChannelSetting.SystemPrompt

		if len(request.Instructions) == 0 {
			if b, err := common.Marshal(systemPrompt); err == nil {
				request.Instructions = b
			} else {
				return nil, err
			}
		} else if info.ChannelSetting.SystemPromptOverride {
			var existing string
			if err := common.Unmarshal(request.Instructions, &existing); err == nil {
				existing = strings.TrimSpace(existing)
				if existing == "" {
					if b, err := common.Marshal(systemPrompt); err == nil {
						request.Instructions = b
					} else {
						return nil, err
					}
				} else {
					if b, err := common.Marshal(systemPrompt + "\n" + existing); err == nil {
						request.Instructions = b
					} else {
						return nil, err
					}
				}
			} else {
				if b, err := common.Marshal(systemPrompt); err == nil {
					request.Instructions = b
				} else {
					return nil, err
				}
			}
		}
	}
	// Codex backend requires the `instructions` field to be present.
	// Keep it consistent with Codex CLI behavior by defaulting to an empty string.
	if len(request.Instructions) == 0 {
		request.Instructions = json.RawMessage(`""`)
	}
	ids, err := resolveFingerprintIDsForRequest(info, clientSessionID(c), time.Now())
	if err != nil {
		return nil, err
	}
	setFingerprintIDs(c, ids)
	if isCompact {
		if ids != nil && ids.mode == dto.CodexFingerprintModeFull {
			if err := validateFullFingerprintClientMetadata(request.ClientMetadata); err != nil {
				return nil, err
			}
		}
	} else if err := applyFingerprintToResponsesRequest(&request, ids); err != nil {
		return nil, err
	}

	raw, err := common.Marshal(request)
	if err != nil {
		return nil, err
	}
	raw, err = ApplyCodexRequestConstraints(raw, isCompact)
	if err != nil {
		return nil, err
	}
	var constrained dto.OpenAIResponsesRequest
	if err := common.Unmarshal(raw, &constrained); err != nil {
		return nil, err
	}
	constrained.ReasoningConversion = request.ReasoningConversion
	return constrained, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if shouldResetCodexFingerprintForAttempt(info) {
		setFingerprintIDs(c, nil)
	}
	preparedBody, closer, err := a.prepareRequestBody(c, info, requestBody)
	if err != nil {
		return nil, err
	}
	if closer != nil {
		defer closer.Close()
	}
	resp, err := channel.DoApiRequest(a, c, info, preparedBody)
	if err != nil {
		return resp, err
	}
	if resp != nil && (info.RelayMode == relayconstant.RelayModeImagesGenerations || info.RelayMode == relayconstant.RelayModeImagesEdits) && resp.StatusCode != http.StatusOK {
		sanitizeCodexImageErrorResponse(resp)
	}
	return resp, nil
}

func shouldResetCodexFingerprintForAttempt(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	if info.RelayMode == relayconstant.RelayModeAlphaSearch {
		return true
	}
	return codexPassThroughBodyEnabled(info)
}

func codexPassThroughBodyEnabled(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled {
		return true
	}
	return info.ChannelMeta != nil && info.ChannelSetting.PassThroughBodyEnabled
}

func (a *Adaptor) prepareRequestBody(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (io.Reader, io.Closer, error) {
	if requestBody == nil || !codexJSONBodyPolicyApplies(c, info) {
		return requestBody, nil, nil
	}

	ids, err := fingerprintIDsForRequest(c, info)
	if err != nil {
		return nil, nil, codexRequestBodyError(err)
	}

	raw, err := readCodexRequestBody(requestBody)
	if err != nil {
		return nil, nil, codexRequestBodyError(err)
	}
	isCompact := info.RelayMode == relayconstant.RelayModeResponsesCompact
	rewritten := raw
	if ids != nil && !isCompact {
		rewritten, err = SanitizeCodexRequestBody(raw, fingerprintFromIDs(ids), ids.mode)
		if err != nil {
			return nil, nil, codexRequestBodyError(err)
		}
	}
	rewritten, err = ApplyCodexRequestConstraints(rewritten, isCompact)
	if err != nil {
		return nil, nil, codexRequestBodyError(err)
	}
	if bytes.Equal(raw, rewritten) {
		return bytes.NewReader(raw), nil, nil
	}
	body, closer, err := relaycommon.NewOutboundJSONBody(rewritten)
	if err != nil {
		return nil, nil, codexRequestBodyError(fmt.Errorf("codex channel: store rewritten request body: %w", err))
	}
	return body, closer, nil
}

func codexRequestBodyError(err error) error {
	return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
}

func codexJSONBodyPolicyApplies(c *gin.Context, info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	switch info.RelayMode {
	case relayconstant.RelayModeChatCompletions,
		relayconstant.RelayModeResponses,
		relayconstant.RelayModeResponsesCompact,
		relayconstant.RelayModeImagesGenerations,
		relayconstant.RelayModeImagesEdits:
	default:
		return false
	}
	if c == nil || c.Request == nil {
		return true
	}
	contentType := strings.ToLower(strings.TrimSpace(c.Request.Header.Get("Content-Type")))
	return contentType == "" || strings.HasPrefix(contentType, "application/json") || strings.HasSuffix(contentType, "+json")
}

func readCodexRequestBody(requestBody io.Reader) ([]byte, error) {
	maxMB := appconstant.MaxRequestBodyMB
	if maxMB <= 0 {
		maxMB = 128
	}
	maxBytes := int64(maxMB) << 20
	if replayable, ok := requestBody.(common.ReplayableBody); ok && replayable.Size() > maxBytes {
		return nil, fmt.Errorf("codex channel: request body exceeds %d MB", maxMB)
	}
	raw, err := io.ReadAll(io.LimitReader(requestBody, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("codex channel: read request body: %w", err)
	}
	if int64(len(raw)) > maxBytes {
		return nil, fmt.Errorf("codex channel: request body exceeds %d MB", maxMB)
	}
	return raw, nil
}

func fingerprintFromIDs(ids *codexFingerprintIDs) *CodexFingerprint {
	if ids == nil {
		return nil
	}
	return &CodexFingerprint{
		InstallationID: ids.installationID,
		SessionID:      ids.sessionID,
		ThreadID:       ids.threadID,
		TurnID:         ids.turnID,
		WindowID:       ids.windowID,
		StartedAtMS:    ids.startedAtMS,
	}
}

func (a *Adaptor) RewritePassThroughBody(c *gin.Context, info *relaycommon.RelayInfo, raw []byte) ([]byte, bool, error) {
	setFingerprintIDs(c, nil)
	ids, err := fingerprintIDsForRequest(c, info)
	if err != nil {
		return nil, false, err
	}
	isCompact := info != nil && info.RelayMode == relayconstant.RelayModeResponsesCompact
	rewritten := raw
	if ids != nil && !isCompact {
		rewritten, err = SanitizeCodexRequestBody(raw, fingerprintFromIDs(ids), ids.mode)
		if err != nil {
			return nil, false, err
		}
	}
	if codexJSONBodyPolicyApplies(c, info) {
		rewritten, err = ApplyCodexRequestConstraints(rewritten, isCompact)
		if err != nil {
			return nil, false, err
		}
	}
	return rewritten, !bytes.Equal(raw, rewritten), nil
}

func sanitizeCodexImageErrorResponse(resp *http.Response) {
	var upstreamBody []byte
	if resp.Body != nil {
		upstreamBody, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
	}
	if len(upstreamBody) > 0 {
		common.SysError(fmt.Sprintf("codex image: upstream returned HTTP %d, body: <redacted>", resp.StatusCode))
	} else {
		common.SysError(fmt.Sprintf("codex image: upstream returned HTTP %d with empty body", resp.StatusCode))
	}

	payload := []byte(`{"error":{"message":"codex image generation failed","type":"upstream_error"}}`)
	resp.Body = io.NopCloser(bytes.NewReader(payload))
	resp.ContentLength = int64(len(payload))
	if resp.Header == nil {
		resp.Header = http.Header{}
	}
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(payload)))
	resp.Header.Del("Transfer-Encoding")
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case relayconstant.RelayModeChatCompletions:
		return RelayChatOverCodex(c, info, resp)
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		return RelayImageOverCodex(c, info, resp)
	case relayconstant.RelayModeAlphaSearch:
		// Alpha search responses are handled by relay.AlphaSearchHelper.
		return nil, types.NewError(errors.New("codex channel: alpha search response should be handled by AlphaSearchHelper"), types.ErrorCodeInvalidRequest)
	case relayconstant.RelayModeResponsesCompact:
		return openai.OaiResponsesCompactionHandler(c, resp)
	case relayconstant.RelayModeResponses:
		if info.IsStream {
			return openai.OaiResponsesStreamHandler(c, info, resp)
		}
		return openai.OaiResponsesHandler(c, info, resp)
	default:
		return nil, types.NewError(errors.New("codex channel: endpoint not supported"), types.ErrorCodeInvalidRequest)
	}
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	var path string
	switch info.RelayMode {
	case relayconstant.RelayModeChatCompletions:
		path = "/backend-api/codex/responses"
	case relayconstant.RelayModeResponses:
		path = "/backend-api/codex/responses"
	case relayconstant.RelayModeResponsesCompact:
		path = "/backend-api/codex/responses/compact"
	case relayconstant.RelayModeAlphaSearch:
		path = "/backend-api/codex/alpha/search"
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		path = "/backend-api/codex/responses"
	default:
		return "", errors.New("codex channel: only /v1/chat/completions, /v1/responses, /v1/responses/compact, /v1/alpha/search and /v1/images endpoints are supported")
	}
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, path, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)

	key := strings.TrimSpace(info.ApiKey)
	if !strings.HasPrefix(key, "{") {
		return errors.New("codex channel: key must be a JSON object")
	}

	oauthKey, err := ParseOAuthKey(key)
	if err != nil {
		return err
	}

	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	accountID := strings.TrimSpace(oauthKey.AccountID)

	if accessToken == "" {
		return errors.New("codex channel: access_token is required")
	}
	if accountID == "" {
		return errors.New("codex channel: account_id is required")
	}

	req.Set("Authorization", "Bearer "+accessToken)
	req.Set("chatgpt-account-id", accountID)

	if req.Get("OpenAI-Beta") == "" {
		req.Set("OpenAI-Beta", "responses=experimental")
	}
	if req.Get("originator") == "" {
		req.Set("originator", "codex_cli_rs")
	}
	service.ApplyCodexInferenceIdentity(*req, service.ResolveCodexClientIdentity())
	ids, err := fingerprintIDsForRequest(c, info)
	if err != nil {
		return err
	}
	applyFingerprintHeaders(*req, ids)

	// chatgpt.com/backend-api/codex/responses is strict about Content-Type.
	// Clients may omit it or include parameters like `application/json; charset=utf-8`,
	// which can be rejected by the upstream. Force the exact media type.
	req.Set("Content-Type", "application/json")
	if info.IsStream || info.RelayMode == relayconstant.RelayModeChatCompletions {
		req.Set("Accept", "text/event-stream")
	} else if req.Get("Accept") == "" {
		req.Set("Accept", "application/json")
	}

	return nil
}
