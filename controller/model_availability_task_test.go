package controller

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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
		{
			name:       "rate limit message",
			err:        errors.New("upstream rate limit exceeded"),
			wantClass:  modelProbeTemporaryFailure,
			wantReason: "temporary_upstream_failure",
		},
		{
			name:       "quota status is an account issue",
			err:        errors.New("quota exceeded"),
			apiErr:     &types.NewAPIError{StatusCode: http.StatusTooManyRequests},
			wantClass:  modelProbeUnknownFailure,
			wantReason: "channel_account_issue",
		},
		{
			name:       "unclassified failure",
			err:        errors.New("upstream returned an unexpected response"),
			wantClass:  modelProbeUnknownFailure,
			wantReason: "unknown_probe_failure",
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

func TestSummarizeModelProbeOutcomes(t *testing.T) {
	tests := []struct {
		name           string
		outcomes       []modelProbeOutcome
		sawUntestable  bool
		wantClass      modelProbeClass
		wantReasonType string
	}{
		{
			name: "all channels officially unsupported",
			outcomes: []modelProbeOutcome{
				{Class: modelProbeOfficialUnsupported, ReasonType: "official_model_unsupported"},
				{Class: modelProbeOfficialUnsupported, ReasonType: "official_model_unsupported"},
			},
			wantClass:      modelProbeOfficialUnsupported,
			wantReasonType: "official_model_unsupported",
		},
		{
			name: "temporary failure takes precedence over unsupported",
			outcomes: []modelProbeOutcome{
				{Class: modelProbeOfficialUnsupported, ReasonType: "official_model_unsupported"},
				{Class: modelProbeTemporaryFailure, ReasonType: "temporary_upstream_failure"},
			},
			wantClass:      modelProbeTemporaryFailure,
			wantReasonType: "temporary_upstream_failure",
		},
		{
			name: "unknown failure takes precedence over unsupported",
			outcomes: []modelProbeOutcome{
				{Class: modelProbeOfficialUnsupported, ReasonType: "official_model_unsupported"},
				{Class: modelProbeUnknownFailure, ReasonType: "channel_account_issue"},
			},
			wantClass:      modelProbeUnknownFailure,
			wantReasonType: "channel_account_issue",
		},
		{
			name:          "untestable channel keeps model available",
			outcomes:      []modelProbeOutcome{{Class: modelProbeOfficialUnsupported}},
			sawUntestable: true,
			wantClass:     modelProbeAvailable,
		},
		{
			name:           "empty result is unknown",
			wantClass:      modelProbeUnknownFailure,
			wantReasonType: "empty_probe_result",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outcome := summarizeModelProbeOutcomes(test.outcomes, test.sawUntestable)
			require.Equal(t, test.wantClass, outcome.Class)
			require.Equal(t, test.wantReasonType, outcome.ReasonType)
		})
	}
}

func TestSaveModelAvailabilityProbeResultRequiresConsecutiveUnsupportedResults(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ModelAvailabilityState{}))

	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	t.Setenv("MODEL_AVAILABILITY_OFFICIAL_UNSUPPORTED_THRESHOLD", "2")

	first := modelProbeOutcome{
		Class:      modelProbeOfficialUnsupported,
		ReasonType: "official_model_unsupported",
		Message:    "model is not supported",
	}
	require.NoError(t, saveModelAvailabilityProbeResult("model-under-test", first))

	state, err := model.GetModelAvailabilityState("model-under-test")
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, model.ModelAvailabilityUnknownFailure, state.Status)
	require.Equal(t, "official_model_unsupported_candidate", state.ReasonType)
	require.Equal(t, 1, state.ConsecutiveFailures)

	require.NoError(t, saveModelAvailabilityProbeResult("model-under-test", first))
	state, err = model.GetModelAvailabilityState("model-under-test")
	require.NoError(t, err)
	require.Equal(t, model.ModelAvailabilityOfficialUnsupported, state.Status)
	require.Equal(t, 2, state.ConsecutiveFailures)

	require.NoError(t, saveModelAvailabilityProbeResult("model-under-test", modelProbeOutcome{Class: modelProbeAvailable}))
	state, err = model.GetModelAvailabilityState("model-under-test")
	require.NoError(t, err)
	require.Equal(t, model.ModelAvailabilityAvailable, state.Status)
	require.Zero(t, state.ConsecutiveFailures)
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
