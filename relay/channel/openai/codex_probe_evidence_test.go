package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexProbeEvidenceKeepsPublicResponsesSanitized(test *testing.T) {
	gin.SetMode(gin.TestMode)
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	test.Cleanup(func() { constant.StreamingTimeout = previousTimeout })
	const message = "The 'gpt-5.4' model is not supported when using Codex with a ChatGPT account. access_token=probe-token-secret account_id=probe-account-sensitive request=probe-request-sensitive"
	for _, scenario := range []struct {
		name            string
		channelTest     bool
		captureEvidence bool
	}{
		{name: "public relay ignores even a bound evidence capture", captureEvidence: true},
		{name: "ordinary channel tests have no evidence capture", channelTest: true},
		{name: "governance probe captures evidence privately", channelTest: true, captureEvidence: true},
	} {
		for _, afterOutput := range []bool{false, true} {
			name := scenario.name + "/before output"
			if afterOutput {
				name = scenario.name + "/after output"
			}
			test.Run(name, func(test *testing.T) {
				ctx := context.Background()
				captured := ""
				if scenario.captureEvidence {
					ctx = WithCodexModelProbeErrorEvidence(ctx, func(evidence string) { captured = evidence })
				}
				recorder := httptest.NewRecorder()
				requestContext, _ := gin.CreateTestContext(recorder)
				requestContext.Request = httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/responses", nil)
				info := &relaycommon.RelayInfo{
					OriginModelName: "gpt-5.4", DisablePing: true, IsChannelTest: scenario.channelTest,
					ChannelMeta:  &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeCodex, UpstreamModelName: "gpt-5.4"},
					StreamStatus: relaycommon.NewStreamStatus(),
				}
				failure, err := common.Marshal(map[string]any{
					"type": "response.failed",
					"response": map[string]any{
						"status": "failed",
						"error": map[string]any{
							"type": "invalid_request_error", "code": "invalid_request_error", "message": message,
							"account_id": "probe-account-sensitive", "request": map[string]string{"prompt": "probe-request-sensitive"},
						},
					},
				})
				require.NoError(test, err)
				body := "data: " + string(failure) + "\n\n"
				if afterOutput {
					body = "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial output\"}\n\n" + body
				}
				response := &http.Response{
					StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)),
					Header: http.Header{"Content-Type": []string{"text/event-stream"}},
				}
				_, apiErr := OaiResponsesStreamHandler(requestContext, info, response)
				require.NotNil(test, apiErr)
				assert.Equal(test, http.StatusBadRequest, apiErr.StatusCode)
				assert.Equal(test, "Codex upstream response failed", apiErr.ToOpenAIError().Message)
				if scenario.channelTest && scenario.captureEvidence {
					assert.Equal(test, message, captured)
				} else {
					assert.Empty(test, captured)
				}
				for _, secret := range []string{"probe-token-secret", "probe-account-sensitive", "probe-request-sensitive", "not supported"} {
					assert.NotContains(test, apiErr.Error(), secret)
					assert.NotContains(test, recorder.Body.String(), secret)
				}
				if afterOutput {
					assert.Contains(test, recorder.Body.String(), "partial output")
					assert.Contains(test, recorder.Body.String(), "Codex upstream response failed")
				} else {
					assert.Empty(test, recorder.Body.String())
				}
			})
		}
	}
}

func TestCodexProbeEvidenceCapturesTopLevelErrorsWithoutChangingPublicResponses(test *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitTokenEncoders()
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	test.Cleanup(func() { constant.StreamingTimeout = previousTimeout })
	const message = "The 'gpt-5.4' model is not supported when using Codex with a ChatGPT account. access_token=fake-probe-token account_id=fake-probe-account request=fake-probe-request"
	for _, event := range []struct {
		name    string
		payload map[string]any
	}{
		{name: "flat error", payload: map[string]any{"type": "error", "code": "invalid_request_error", "message": message}},
		{name: "nested error", payload: map[string]any{"type": "error", "error": map[string]any{
			"type": "invalid_request_error", "code": "invalid_request_error", "message": message,
		}}},
	} {
		payload, err := common.Marshal(event.payload)
		require.NoError(test, err)
		for _, scenario := range []struct {
			name        string
			channelType int
			channelTest bool
		}{
			{name: "governance probe", channelType: constant.ChannelTypeCodex, channelTest: true},
			{name: "ordinary relay", channelType: constant.ChannelTypeCodex},
			{name: "other provider channel test", channelType: constant.ChannelTypeOpenAI, channelTest: true},
		} {
			for _, afterOutput := range []bool{false, true} {
				name := event.name + "/" + scenario.name + "/before output"
				if afterOutput {
					name = event.name + "/" + scenario.name + "/after output"
				}
				test.Run(name, func(test *testing.T) {
					var publicBody string
					for _, captureEvidence := range []bool{false, true} {
						ctx := context.Background()
						captured := ""
						if captureEvidence {
							ctx = WithCodexModelProbeErrorEvidence(ctx, func(evidence string) { captured = evidence })
						}
						recorder := httptest.NewRecorder()
						requestContext, _ := gin.CreateTestContext(recorder)
						requestContext.Request = httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/responses", nil)
						info := &relaycommon.RelayInfo{
							DisablePing: true, IsChannelTest: scenario.channelTest,
							ChannelMeta:  &relaycommon.ChannelMeta{ChannelType: scenario.channelType, UpstreamModelName: "gpt-5.4"},
							StreamStatus: relaycommon.NewStreamStatus(),
						}
						body := "data: " + string(payload) + "\n\n"
						if afterOutput {
							body = "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial output\"}\n\n" + body
						}
						response := &http.Response{
							StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)),
							Header: http.Header{"Content-Type": []string{"text/event-stream"}},
						}
						_, apiErr := OaiResponsesStreamHandler(requestContext, info, response)
						if scenario.channelType == constant.ChannelTypeCodex {
							require.NotNil(test, apiErr)
							assert.Equal(test, http.StatusInternalServerError, apiErr.StatusCode)
							assert.Equal(test, "Codex upstream response failed", apiErr.ToOpenAIError().Message)
							for _, secret := range []string{"fake-probe-token", "fake-probe-account", "fake-probe-request", "not supported"} {
								assert.NotContains(test, apiErr.Error(), secret)
								assert.NotContains(test, recorder.Body.String(), secret)
							}
							if !afterOutput {
								assert.Empty(test, recorder.Body.String())
							}
						} else {
							require.Nil(test, apiErr)
						}
						if captureEvidence && scenario.channelTest && scenario.channelType == constant.ChannelTypeCodex {
							assert.Equal(test, message, captured)
						} else {
							assert.Empty(test, captured)
						}
						if captureEvidence {
							assert.Equal(test, publicBody, recorder.Body.String())
						} else {
							publicBody = recorder.Body.String()
						}
					}
				})
			}
		}
	}
}
