package codex

import (
	"net/http"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

var codexDropHeaders = []string{
	"Cookie",
	"traceparent",
	"tracestate",
	"baggage",
	"Accept-Language",
	"OpenAI-Locale",
	"OpenAI-Timeout-Ms",
	"X-Codex-Beta-Features",
	"X-Codex-Turn-State",
	"X-Codex-Attestation",
}

var codexIdentityDropHeaders = []string{
	"User-Agent",
	"OpenAI-Client",
	"OpenAI-Client-Version",
	"X-OpenAI-Client",
	"X-OpenAI-Client-Version",
	"X-Codex-Client-Version",
	"X-Codex-CLI-Version",
	"X-Codex-Version",
	"Codex-Version",
}

func (a *Adaptor) FinalizeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *http.Request) error {
	return finalizeCodexRequest(c, req, info)
}

func FinalizeCodexRequest(req *http.Request, info *relaycommon.RelayInfo) error {
	return finalizeCodexRequest(nil, req, info)
}

func finalizeCodexRequest(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	if req == nil {
		return nil
	}
	if req.Header == nil {
		req.Header = http.Header{}
	}

	for _, name := range codexDropHeaders {
		req.Header.Del(name)
	}
	enforceClientIdentity := service.IsCodexClientIdentityEnforced()
	if enforceClientIdentity {
		for _, name := range codexIdentityDropHeaders {
			req.Header.Del(name)
		}
		req.Header.Set("User-Agent", "")
	}
	dropUnknownCodexHeaders(req.Header, true)

	if info == nil || info.ChannelMeta == nil {
		req.Header.Del("Authorization")
		req.Header.Del("chatgpt-account-id")
		req.Header.Del("OpenAI-Beta")
		req.Header.Del("X-Codex-Turn-Metadata")
		dropUnknownCodexHeaders(req.Header, false)
		return codexPolicyAPIError(&codexPolicyError{"codex channel: relay info is required"})
	}

	oauthKey, err := ParseOAuthKey(strings.TrimSpace(info.ApiKey))
	if err != nil {
		return codexPolicyAPIError(err)
	}
	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	accountID := strings.TrimSpace(oauthKey.AccountID)
	if accessToken == "" {
		return codexPolicyAPIError(errAccessTokenRequired())
	}
	if accountID == "" {
		return codexPolicyAPIError(errAccountIDRequired())
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("chatgpt-account-id", accountID)
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	if enforceClientIdentity {
		req.Header.Set("originator", "codex_cli_rs")
		service.ApplyCodexInferenceIdentitySnapshot(req.Header, service.ResolveCodexClientIdentity())
	}
	req.Header.Set("Content-Type", "application/json")
	if info.IsStream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}

	ids, err := fingerprintIDsForRequest(c, info)
	if err != nil {
		return err
	}
	if ids == nil {
		req.Header.Del("X-Codex-Turn-Metadata")
		dropUnknownCodexHeaders(req.Header, false)
		return nil
	}
	applyFingerprintHeaders(req.Header, ids)
	return nil
}

func codexPolicyAPIError(err error) error {
	return types.NewErrorWithStatusCode(
		err,
		types.ErrorCodeChannelInvalidKey,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)
}

func dropUnknownCodexHeaders(header http.Header, preserveTurnMetadata bool) {
	for name := range header {
		lower := strings.ToLower(name)
		if preserveTurnMetadata && lower == "x-codex-turn-metadata" {
			continue
		}
		if strings.HasPrefix(lower, "x-codex-") {
			header.Del(name)
		}
	}
}

func errAccessTokenRequired() error {
	return &codexPolicyError{"codex channel: access_token is required"}
}

func errAccountIDRequired() error {
	return &codexPolicyError{"codex channel: account_id is required"}
}

type codexPolicyError struct {
	message string
}

func (e *codexPolicyError) Error() string {
	return e.message
}
