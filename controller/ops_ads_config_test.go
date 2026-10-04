package controller

import "testing"

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
	if _, configured := opsAdsCredsFromEnv(); configured {
		t.Fatal("ads integration enabled without campaign and landing scope")
	}

	t.Setenv("GOOGLE_ADS_CAMPAIGN_PREFIX", "brand")
	t.Setenv("GOOGLE_ADS_LANDING_HOST", "https://www.brand.example/path")
	creds, configured := opsAdsCredsFromEnv()
	if !configured {
		t.Fatal("ads integration did not enable with complete configuration")
	}
	if creds.customerId != "1234567890" || creds.campaignPrefix != "brand" || creds.landingHost != "www.brand.example" {
		t.Fatalf("normalized ads config = %+v", creds)
	}
}

func TestOpsAdsCredsRejectUnsafeCampaignPrefix(t *testing.T) {
	t.Setenv("GOOGLE_ADS_DEVELOPER_TOKEN", "developer")
	t.Setenv("GOOGLE_ADS_CLIENT_ID", "client")
	t.Setenv("GOOGLE_ADS_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_ADS_REFRESH_TOKEN", "refresh")
	t.Setenv("GOOGLE_ADS_CUSTOMER_ID", "1234567890")
	t.Setenv("GOOGLE_ADS_CAMPAIGN_PREFIX", "brand' OR 1=1")
	t.Setenv("GOOGLE_ADS_LANDING_HOST", "brand.example")
	if _, configured := opsAdsCredsFromEnv(); configured {
		t.Fatal("unsafe campaign prefix enabled ads integration")
	}
}
