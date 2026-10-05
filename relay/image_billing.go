package relay

import "github.com/QuantumNous/new-api/relaykit/dto"

func normalizeImageUsageForBilling(usage *dto.Usage, usePrice bool) {
	if !usePrice || usage == nil || usage.UsageSource == dto.UsageSourceCodexImage {
		return
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = 1
	}
	if usage.PromptTokens == 0 {
		usage.PromptTokens = 1
	}
}
