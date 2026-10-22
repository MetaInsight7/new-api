package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpsAdsCredsRequireReportScope(t *testing.T) {
	for key, value := range map[string]string{
		"GOOGLE_ADS_DEVELOPER_TOKEN": "developer",
		"GOOGLE_ADS_CLIENT_ID":       "client",
		"GOOGLE_ADS_CLIENT_SECRET":   "secret",
		"GOOGLE_ADS_REFRESH_TOKEN":   "refresh",
		"GOOGLE_ADS_CUSTOMER_ID":     "123-456-7890",
	} {
		t.Setenv(key, value)
	}
	t.Setenv("GOOGLE_ADS_CAMPAIGN_PREFIX", "")
	t.Setenv("GOOGLE_ADS_LANDING_HOST", "")
	_, configured := opsAdsCredsFromEnv()
	assert.False(t, configured)

	t.Setenv("GOOGLE_ADS_CAMPAIGN_PREFIX", "brand")
	t.Setenv("GOOGLE_ADS_LANDING_HOST", "https://www.brand.example/path")
	creds, configured := opsAdsCredsFromEnv()
	require.True(t, configured)
	assert.Equal(t, "1234567890", creds.customerId)
	assert.Equal(t, "brand", creds.campaignPrefix)
	assert.Equal(t, "www.brand.example", creds.landingHost)
}

func TestOpsAdsCredsRejectUnsafeCampaignPrefix(t *testing.T) {
	t.Setenv("GOOGLE_ADS_DEVELOPER_TOKEN", "developer")
	t.Setenv("GOOGLE_ADS_CLIENT_ID", "client")
	t.Setenv("GOOGLE_ADS_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_ADS_REFRESH_TOKEN", "refresh")
	t.Setenv("GOOGLE_ADS_CUSTOMER_ID", "1234567890")
	t.Setenv("GOOGLE_ADS_CAMPAIGN_PREFIX", "brand' OR 1=1")
	t.Setenv("GOOGLE_ADS_LANDING_HOST", "brand.example")
	_, configured := opsAdsCredsFromEnv()
	assert.False(t, configured)
}
