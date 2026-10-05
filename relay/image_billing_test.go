package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeImageUsageForBillingKeepsCodexUnknownUsageFree(t *testing.T) {
	usage := &dto.Usage{UsageSource: dto.UsageSourceCodexImage}

	normalizeImageUsageForBilling(usage, true)

	assert.Zero(t, usage.PromptTokens)
	assert.Zero(t, usage.TotalTokens)
}

func TestNormalizeImageUsageForBillingPreservesLegacyImageFallback(t *testing.T) {
	usage := &dto.Usage{}

	normalizeImageUsageForBilling(usage, true)

	assert.Equal(t, 1, usage.PromptTokens)
	assert.Equal(t, 1, usage.TotalTokens)
}

func TestNormalizeImageUsageForBillingDoesNotEstimateTokenPricedUsage(t *testing.T) {
	usage := &dto.Usage{}

	normalizeImageUsageForBilling(usage, false)

	assert.Zero(t, usage.PromptTokens)
	assert.Zero(t, usage.TotalTokens)
}
