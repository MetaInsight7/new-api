package controller

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/require"
)

func TestClassifyModelProbeError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		apiErr     *types.NewAPIError
		wantClass  modelProbeClass
		wantReason string
	}{
		{
			name:       "officially unsupported",
			err:        errors.New("model_not_found: model does not exist"),
			wantClass:  modelProbeOfficialUnsupported,
			wantReason: "official_model_unsupported",
		},
		{
			name:       "account issue is not model retirement",
			err:        errors.New("invalid api key"),
			wantClass:  modelProbeUnknownFailure,
			wantReason: "channel_account_issue",
		},
		{
			name:       "temporary status",
			apiErr:     &types.NewAPIError{StatusCode: http.StatusServiceUnavailable},
			wantClass:  modelProbeTemporaryFailure,
			wantReason: "temporary_upstream_failure",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outcome := classifyModelProbeError(test.err, test.apiErr)
			require.Equal(t, test.wantClass, outcome.Class)
			require.Equal(t, test.wantReason, outcome.ReasonType)
		})
	}
}

func TestClassifyModelProbeModality(t *testing.T) {
	require.Equal(t, modelProbeModalityEmbedding, classifyModelProbeModality(0, "text-embedding-3-small"))
	require.Equal(t, modelProbeModalityImage, classifyModelProbeModality(0, "sdxl"))
	require.Equal(t, modelProbeModalityRerank, classifyModelProbeModality(constant.ChannelTypeJina, "jina-reranker-v2"))
	require.Equal(t, modelProbeModalityUntestable, classifyModelProbeModality(constant.ChannelTypeSunoAPI, "suno"))
	require.Equal(t, modelProbeModalityChat, classifyModelProbeModality(0, "gpt-4o-mini"))
}

func TestValidatePongTestResponseBody(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "chat response", body: `{"choices":[{"message":{"content":"pong"}}]}`},
		{name: "stream response", body: "data: {\"choices\":[{\"delta\":{\"content\":\"pong\"}}]}\n\ndata: [DONE]"},
		{name: "successful non pong text", body: `{"choices":[{"message":{"content":"ready"}}]}`},
		{name: "empty response", body: "", wantErr: true},
		{name: "plain empty content", body: "   ", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validatePongTestResponseBody([]byte(test.body))
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
