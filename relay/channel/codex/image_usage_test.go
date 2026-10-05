package codex

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildCodexImageUsageUsesAuthoritativeTokens(t *testing.T) {
	usage := buildCodexImageUsage(gjson.Parse(`{
		"input_tokens": 21,
		"output_tokens": 196,
		"total_tokens": 217,
		"input_tokens_details": {"text_tokens": 17, "image_tokens": 4},
		"output_tokens_details": {"text_tokens": 0, "image_tokens": 196}
	}`), true)

	assert.Equal(t, dto.UsageSourceCodexImage, usage.UsageSource)
	assert.Equal(t, dto.BillingUsageSemanticOpenAI, usage.UsageSemantic)
	assert.Equal(t, 21, usage.PromptTokens)
	assert.Equal(t, 196, usage.CompletionTokens)
	assert.Equal(t, 217, usage.TotalTokens)
	assert.Equal(t, 4, usage.PromptTokensDetails.ImageTokens)
	assert.Equal(t, 196, usage.CompletionTokenDetails.ImageTokens)
	require.NotNil(t, usage.BillingUsage)
	assert.Equal(t, dto.BillingUsageSourceOAIResponses, usage.BillingUsage.Source)
	require.NotNil(t, usage.BillingUsage.OpenAIUsage)
	assert.Equal(t, 217, usage.BillingUsage.OpenAIUsage.TotalTokens)
}

func TestBuildCodexImageUsagePreservesPartialAuthoritativeUsage(t *testing.T) {
	usage := buildCodexImageUsage(gjson.Parse(`{
		"input_tokens": 21,
		"input_tokens_details": {"image_tokens": 4}
	}`), true)

	assert.Equal(t, 21, usage.PromptTokens)
	assert.Zero(t, usage.CompletionTokens)
	assert.Equal(t, 21, usage.TotalTokens)
	require.NotNil(t, usage.BillingUsage)
	assert.Equal(t, dto.BillingUsageSemanticOpenAI, usage.BillingUsage.Semantic)
}

func TestBuildCodexImageUsageDoesNotGuessMissingUsage(t *testing.T) {
	usage := buildCodexImageUsage(gjson.Result{}, false)

	assert.Equal(t, dto.UsageSourceCodexImage, usage.UsageSource)
	assert.Empty(t, usage.UsageSemantic)
	assert.Zero(t, usage.PromptTokens)
	assert.Zero(t, usage.CompletionTokens)
	assert.Zero(t, usage.TotalTokens)
	assert.Nil(t, usage.BillingUsage)
}
