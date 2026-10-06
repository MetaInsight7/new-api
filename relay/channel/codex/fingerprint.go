package codex

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	fingerprintContextKey    = "codex_fingerprint_ids"
	maxCodexMetadataBytes    = 64 * 1024
	maxCodexMetadataDepth    = 10
	maxCodexRequestJSONDepth = 100
)

type CodexFingerprint struct {
	InstallationID string
	SessionID      string
	ThreadID       string
	TurnID         string
	WindowID       string
	StartedAtMS    int64
}

type codexFingerprintIDs struct {
	mode, installationID, sessionID, threadID, turnID, windowID string
	scope                                                       string
	startedAtMS                                                 int64
}

func clientSessionID(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	if value := strings.TrimSpace(c.Request.Header.Get("session-id")); value != "" {
		return value
	}
	return strings.TrimSpace(c.Request.Header.Get("session_id"))
}

func setFingerprintIDs(c *gin.Context, ids *codexFingerprintIDs) {
	if c != nil {
		c.Set(fingerprintContextKey, ids)
	}
}

func fingerprintMode(info *relaycommon.RelayInfo) string {
	if info == nil || info.ChannelMeta == nil {
		return dto.CodexFingerprintModeOff
	}
	return info.ChannelSetting.NormalizedCodexFingerprintMode()
}

func stableCodexID(seed, deploymentNamespace, label string) string {
	material := fmt.Sprintf("new-api:codex-fingerprint:v3:%s:%s:%s", deploymentNamespace, seed, label)
	hash := sha256.Sum256([]byte(material))
	identifier := hash[:16]
	identifier[6] = identifier[6]&0x0f | 0x40
	identifier[8] = identifier[8]&0x3f | 0x80
	return uuid.UUID(identifier).String()
}

func ResolveCodexFingerprint(info *relaycommon.RelayInfo, originalSession string, now time.Time) (*CodexFingerprint, error) {
	if info == nil || fingerprintMode(info) == dto.CodexFingerprintModeOff {
		return nil, nil
	}
	if info.ChannelMeta == nil || strings.TrimSpace(info.ChannelMeta.CodexFingerprintSeed) == "" {
		return nil, errors.New("codex channel: fingerprint seed is required")
	}
	seed := strings.TrimSpace(info.ChannelMeta.CodexFingerprintSeed)
	parsedSeed, err := uuid.Parse(seed)
	if err != nil || parsedSeed == uuid.Nil || seed != parsedSeed.String() {
		return nil, errors.New("codex channel: fingerprint seed is invalid")
	}
	deploymentNamespace := strings.TrimSpace(os.Getenv("CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE"))
	if deploymentNamespace == "" {
		return nil, errors.New("codex channel: fingerprint namespace is required")
	}

	mode := fingerprintMode(info)
	sessionID := stableCodexID(seed, deploymentNamespace, "session")
	threadID := sessionID
	if mode == dto.CodexFingerprintModeSession {
		clientSession := strings.TrimSpace(originalSession)
		if clientSession == "" {
			clientSession = "default"
		}
		threadID = stableCodexID(seed, deploymentNamespace, "thread:"+clientSession)
	}
	turnID, err := uuid.NewV7()
	if err != nil {
		return nil, errors.New("codex channel: create turn id")
	}
	return &CodexFingerprint{
		InstallationID: stableCodexID(seed, deploymentNamespace, "installation"),
		SessionID:      sessionID,
		ThreadID:       threadID,
		TurnID:         turnID.String(),
		WindowID:       stableCodexID(seed, deploymentNamespace, "window:"+threadID),
		StartedAtMS:    now.UnixMilli(),
	}, nil
}

func codexFingerprintFromPublic(mode string, fingerprint *CodexFingerprint) *codexFingerprintIDs {
	if fingerprint == nil {
		return nil
	}
	return &codexFingerprintIDs{
		mode: mode, installationID: fingerprint.InstallationID, sessionID: fingerprint.SessionID,
		threadID: fingerprint.ThreadID, turnID: fingerprint.TurnID, windowID: fingerprint.WindowID,
		startedAtMS: fingerprint.StartedAtMS,
	}
}

func resolveFingerprintIDsForRequest(info *relaycommon.RelayInfo, clientSession string, now time.Time) (*codexFingerprintIDs, error) {
	mode := fingerprintMode(info)
	if mode == dto.CodexFingerprintModeOff {
		return nil, nil
	}
	fingerprint, err := ResolveCodexFingerprint(info, clientSession, now)
	if err != nil {
		return nil, err
	}
	ids := codexFingerprintFromPublic(mode, fingerprint)
	ids.scope = codexFingerprintScope(info)
	return ids, nil
}

func fingerprintIDsForRequest(c *gin.Context, info *relaycommon.RelayInfo) (*codexFingerprintIDs, error) {
	scope := codexFingerprintScope(info)
	if c != nil {
		if value, ok := c.Get(fingerprintContextKey); ok {
			if ids, ok := value.(*codexFingerprintIDs); ok && ids != nil && ids.scope == scope {
				return ids, nil
			}
		}
	}
	ids, err := resolveFingerprintIDsForRequest(info, clientSessionID(c), time.Now())
	if err == nil {
		setFingerprintIDs(c, ids)
	}
	return ids, err
}

func codexFingerprintScope(info *relaycommon.RelayInfo) string {
	if info == nil || info.ChannelMeta == nil {
		return ""
	}
	return fmt.Sprintf("%d:%d:%s:%s", info.ChannelId, info.ChannelMultiKeyIndex, strings.TrimSpace(info.CodexFingerprintSeed), fingerprintMode(info))
}

func rewriteTurnMetadata(raw string, ids *codexFingerprintIDs) string {
	if ids == nil || strings.TrimSpace(raw) == "" {
		return raw
	}
	var metadata map[string]any
	if err := common.Unmarshal([]byte(raw), &metadata); err != nil || metadata == nil {
		metadata = make(map[string]any)
	}
	metadata["installation_id"] = ids.installationID
	if ids.mode != dto.CodexFingerprintModeDevice {
		metadata["session_id"] = ids.sessionID
		metadata["thread_id"] = ids.threadID
		metadata["turn_id"] = ids.turnID
		metadata["window_id"] = ids.windowID
		metadata["turn_started_at_unix_ms"] = ids.startedAtMS
	}
	result, err := common.Marshal(metadata)
	if err != nil {
		return raw
	}
	return string(result)
}

type fingerprintMetadataField struct {
	name  string
	value any
}

func fingerprintMetadataFields(ids *codexFingerprintIDs) []fingerprintMetadataField {
	if ids == nil {
		return nil
	}
	fields := []fingerprintMetadataField{{name: "x-codex-installation-id", value: ids.installationID}}
	if ids.mode != dto.CodexFingerprintModeDevice {
		fields = append(fields,
			fingerprintMetadataField{name: "session_id", value: ids.sessionID},
			fingerprintMetadataField{name: "thread_id", value: ids.threadID},
			fingerprintMetadataField{name: "turn_id", value: ids.turnID},
			fingerprintMetadataField{name: "x-codex-window-id", value: ids.windowID},
			fingerprintMetadataField{name: "turn_started_at_unix_ms", value: strconv.FormatInt(ids.startedAtMS, 10)},
		)
	}
	return fields
}

func fullFingerprintMetadata(ids *codexFingerprintIDs) map[string]any {
	metadata := make(map[string]any)
	for _, field := range fingerprintMetadataFields(ids) {
		metadata[field.name] = field.value
	}
	return metadata
}

func applyFingerprintMetadata(metadata map[string]any, ids *codexFingerprintIDs) bool {
	if metadata == nil || ids == nil {
		return false
	}
	for _, field := range fingerprintMetadataFields(ids) {
		metadata[field.name] = field.value
	}
	if raw, ok := metadata["x-codex-turn-metadata"].(string); ok {
		metadata["x-codex-turn-metadata"] = rewriteTurnMetadata(raw, ids)
	}
	return true
}

func applyFingerprintBody(body map[string]any, ids *codexFingerprintIDs) bool {
	if body == nil || ids == nil {
		return false
	}
	if ids.mode == dto.CodexFingerprintModeFull {
		raw, err := common.Marshal(body)
		if err != nil {
			return false
		}
		rewritten, err := SanitizeCodexRequestBody(raw, &CodexFingerprint{
			InstallationID: ids.installationID, SessionID: ids.sessionID, ThreadID: ids.threadID,
			TurnID: ids.turnID, WindowID: ids.windowID, StartedAtMS: ids.startedAtMS,
		}, ids.mode)
		if err != nil {
			return false
		}
		var sanitized map[string]any
		if err := common.Unmarshal(rewritten, &sanitized); err != nil || sanitized == nil {
			return false
		}
		for key := range body {
			delete(body, key)
		}
		for key, value := range sanitized {
			body[key] = value
		}
		return true
	}
	metadata, _ := body["client_metadata"].(map[string]any)
	if metadata == nil {
		metadata = make(map[string]any)
	}
	body["client_metadata"] = metadata
	return applyFingerprintMetadata(metadata, ids)
}

func applyFingerprintToResponsesRequest(request *dto.OpenAIResponsesRequest, ids *codexFingerprintIDs) error {
	if request == nil || ids == nil {
		return nil
	}
	var metadata map[string]any
	if len(request.ClientMetadata) > 0 {
		if err := common.Unmarshal(request.ClientMetadata, &metadata); err != nil || metadata == nil {
			metadata = make(map[string]any)
		}
	} else {
		metadata = make(map[string]any)
	}
	if ids.mode == dto.CodexFingerprintModeFull {
		originalSession, _ := metadata["session_id"].(string)
		var cacheKey string
		_ = common.Unmarshal(request.PromptCacheKey, &cacheKey)
		if strings.TrimSpace(originalSession) != "" && cacheKey == strings.TrimSpace(originalSession) {
			request.PromptCacheKey, _ = common.Marshal(ids.sessionID)
		}
		metadata = fullFingerprintMetadata(ids)
	} else if !applyFingerprintMetadata(metadata, ids) {
		return errors.New("codex channel: sanitize fingerprint metadata")
	}
	encoded, err := common.Marshal(metadata)
	if err != nil {
		return errors.New("codex channel: marshal fingerprint metadata")
	}
	request.ClientMetadata = encoded
	return nil
}

func applyFingerprintHeaders(headers http.Header, ids *codexFingerprintIDs) {
	if headers == nil || ids == nil {
		return
	}
	headers.Set("x-codex-installation-id", ids.installationID)
	if ids.mode == dto.CodexFingerprintModeDevice {
		if raw := headers.Get("x-codex-turn-metadata"); raw != "" {
			headers.Set("x-codex-turn-metadata", rewriteTurnMetadata(raw, ids))
		}
		return
	}
	headers.Set("x-codex-window-id", ids.windowID)
	headers.Set("x-client-request-id", ids.threadID)
	headers.Set("session-id", ids.sessionID)
	headers.Set("session_id", ids.sessionID)
	headers.Set("thread-id", ids.threadID)
	if raw := headers.Get("x-codex-turn-metadata"); raw != "" {
		headers.Set("x-codex-turn-metadata", rewriteTurnMetadata(raw, ids))
	}
}

func SanitizeCodexRequestBody(raw []byte, fingerprint *CodexFingerprint, mode string) ([]byte, error) {
	if fingerprint == nil || mode == dto.CodexFingerprintModeOff {
		return raw, nil
	}
	ids := codexFingerprintFromPublic(mode, fingerprint)
	if ids == nil {
		return raw, nil
	}
	if err := validateCodexRequestBodyRaw(raw); err != nil {
		return nil, err
	}
	if mode != dto.CodexFingerprintModeFull {
		rewritten, _, err := applyFingerprintBodyRaw(raw, ids)
		return rewritten, err
	}
	if len(raw) == 0 {
		return nil, errors.New("codex channel: full fingerprint request body is empty")
	}
	if err := validateFullFingerprintMetadataRaw(raw); err != nil {
		return nil, err
	}
	var body map[string]any
	if err := common.Unmarshal(raw, &body); err != nil || body == nil {
		return nil, errors.New("codex channel: invalid full fingerprint request json")
	}
	originalSession := ""
	if rawMetadata, ok := body["client_metadata"]; ok {
		metadata, ok := rawMetadata.(map[string]any)
		if !ok {
			return nil, errors.New("codex channel: full fingerprint client_metadata must be an object")
		}
		if session, ok := metadata["session_id"].(string); ok {
			originalSession = strings.TrimSpace(session)
		}
	}
	if cacheKey, ok := body["prompt_cache_key"].(string); ok && originalSession != "" && cacheKey == originalSession {
		body["prompt_cache_key"] = ids.sessionID
	}
	body["client_metadata"] = fullFingerprintMetadata(ids)
	result, err := common.Marshal(body)
	if err != nil {
		return nil, errors.New("codex channel: marshal full fingerprint request body")
	}
	return result, nil
}

func validateFullFingerprintMetadataRaw(raw []byte) error {
	metadata := gjson.GetBytes(raw, "client_metadata")
	if !metadata.Exists() {
		return rejectDuplicateJSONKeys(raw, maxCodexRequestJSONDepth)
	}
	if !metadata.IsObject() {
		return errors.New("codex channel: full fingerprint client_metadata must be an object")
	}
	if len(metadata.Raw) > maxCodexMetadataBytes {
		return errors.New("codex channel: full fingerprint client_metadata is too large")
	}
	if exceedsJSONRawDepth([]byte(metadata.Raw), maxCodexMetadataDepth) {
		return errors.New("codex channel: full fingerprint client_metadata is too deeply nested")
	}
	return rejectDuplicateJSONKeys(raw, maxCodexRequestJSONDepth)
}

func validateCodexRequestBodyRaw(raw []byte) error {
	if len(raw) == 0 {
		return errors.New("codex channel: request body is empty")
	}
	if err := rejectDuplicateJSONKeys(raw, maxCodexRequestJSONDepth); err != nil {
		return err
	}
	metadata := gjson.GetBytes(raw, "client_metadata")
	if !metadata.Exists() {
		return nil
	}
	if !metadata.IsObject() {
		return errors.New("codex channel: client_metadata must be an object")
	}
	if len(metadata.Raw) > maxCodexMetadataBytes {
		return errors.New("codex channel: client_metadata is too large")
	}
	if exceedsJSONRawDepth([]byte(metadata.Raw), maxCodexMetadataDepth) {
		return errors.New("codex channel: client_metadata is too deeply nested")
	}
	return nil
}

func stripCompactOriginalMetadataRaw(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}
	root := gjson.ParseBytes(body)
	if !gjson.ValidBytes(body) || !root.IsObject() {
		return body, false, nil
	}
	rewritten := body
	changed := false
	for _, key := range []string{"client_metadata", "metadata"} {
		if !gjson.GetBytes(rewritten, key).Exists() {
			continue
		}
		var err error
		rewritten, err = sjson.DeleteBytes(rewritten, key)
		if err != nil {
			return body, false, fmt.Errorf("codex channel: strip compact original metadata: %w", err)
		}
		changed = true
	}
	return rewritten, changed, nil
}

func validateCompactPassThroughFullBody(raw []byte) error {
	if len(raw) == 0 {
		return errors.New("codex channel: full fingerprint compact request body is empty")
	}
	if err := validateFullFingerprintMetadataRaw(raw); err != nil {
		return err
	}
	var body map[string]any
	if err := common.Unmarshal(raw, &body); err != nil || body == nil {
		return errors.New("codex channel: invalid full fingerprint compact request json")
	}
	if metadata, ok := body["client_metadata"]; ok {
		if _, ok := metadata.(map[string]any); !ok {
			return errors.New("codex channel: full fingerprint compact client_metadata must be an object")
		}
	}
	return nil
}

func rejectDuplicateJSONKeys(raw []byte, maxDepth int) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := consumeJSONValue(decoder, 0, maxDepth); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return errors.New("multiple json values")
		}
		return err
	}
	return nil
}

func consumeJSONValue(decoder *json.Decoder, depth, maxDepth int) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if depth >= maxDepth {
		return errors.New("json nesting exceeds maximum depth")
	}
	nestedDepth := depth + 1
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("json object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate json key %q", key)
			}
			seen[key] = struct{}{}
			if err := consumeJSONValue(decoder, nestedDepth, maxDepth); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("json object is not closed")
		}
	case '[':
		for decoder.More() {
			if err := consumeJSONValue(decoder, nestedDepth, maxDepth); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("json array is not closed")
		}
	case '}', ']':
		return errors.New("unexpected json closing delimiter")
	}
	return nil
}

func exceedsJSONRawDepth(raw []byte, maxDepth int) bool {
	depth := 0
	inString := false
	escaped := false
	for _, value := range raw {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if value == '\\' {
				escaped = true
				continue
			}
			if value == '"' {
				inString = false
			}
			continue
		}
		switch value {
		case '"':
			inString = true
		case '{', '[':
			depth++
			if depth > maxDepth {
				return true
			}
		case '}', ']':
			depth--
		}
	}
	return false
}

func applyFingerprintBodyRaw(body []byte, ids *codexFingerprintIDs) ([]byte, bool, error) {
	if len(body) == 0 || ids == nil {
		return body, false, nil
	}
	root := gjson.ParseBytes(body)
	if !gjson.ValidBytes(body) || !root.IsObject() {
		return body, false, nil
	}
	rewritten := body
	if existing := gjson.GetBytes(body, "client_metadata"); !existing.IsObject() {
		var err error
		rewritten, err = sjson.SetRawBytes(rewritten, "client_metadata", []byte(`{}`))
		if err != nil {
			return body, false, errors.New("codex channel: initialize fingerprint metadata")
		}
	}
	for _, field := range fingerprintMetadataFields(ids) {
		var err error
		rewritten, err = sjson.SetBytes(rewritten, "client_metadata."+field.name, field.value)
		if err != nil {
			return body, false, errors.New("codex channel: write fingerprint metadata")
		}
	}
	if raw := gjson.GetBytes(rewritten, "client_metadata.x-codex-turn-metadata"); raw.Type == gjson.String {
		var err error
		rewritten, err = sjson.SetBytes(rewritten, "client_metadata.x-codex-turn-metadata", rewriteTurnMetadata(raw.String(), ids))
		if err != nil {
			return body, false, errors.New("codex channel: write fingerprint turn metadata")
		}
	}
	return rewritten, true, nil
}
