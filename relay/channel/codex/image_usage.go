package codex

import (
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/tidwall/gjson"
)

func buildCodexImageUsage(imageUsage gjson.Result, hasUsage bool) *dto.Usage {
	usage := &dto.Usage{UsageSource: dto.UsageSourceCodexImage}
	if !hasUsage {
		return usage
	}

	usage.PromptTokens = nonNegativeInt(imageUsage.Get("input_tokens").Int())
	usage.CompletionTokens = nonNegativeInt(imageUsage.Get("output_tokens").Int())
	usage.TotalTokens = nonNegativeInt(imageUsage.Get("total_tokens").Int())
	if usage.TotalTokens < usage.PromptTokens+usage.CompletionTokens {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	inputDetails := imageUsage.Get("input_tokens_details")
	usage.PromptTokensDetails.TextTokens = nonNegativeInt(inputDetails.Get("text_tokens").Int())
	usage.PromptTokensDetails.ImageTokens = nonNegativeInt(inputDetails.Get("image_tokens").Int())

	outputDetails := imageUsage.Get("output_tokens_details")
	usage.CompletionTokenDetails.TextTokens = nonNegativeInt(outputDetails.Get("text_tokens").Int())
	usage.CompletionTokenDetails.ImageTokens = nonNegativeInt(outputDetails.Get("image_tokens").Int())
	if usage.CompletionTokenDetails.ImageTokens == 0 {
		usage.CompletionTokenDetails.ImageTokens = nonNegativeInt(imageUsage.Get("image_tokens").Int())
	}
	usage.UsageSemantic = dto.BillingUsageSemanticOpenAI
	usage.BillingUsage = dto.NewOpenAIResponsesBillingUsage(usage)
	return usage
}

func nonNegativeInt(value int64) int {
	if value <= 0 {
		return 0
	}
	return int(value)
}
