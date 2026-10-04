package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const maxAdsAttributionLength = 4096

var allowedAdsAttributionKeys = map[string]struct{}{
	"account": {}, "ad_group": {}, "ad_group_id": {}, "aff": {}, "campaign": {},
	"campaign_id": {}, "captured_at": {}, "country": {}, "creative": {},
	"creative_id": {}, "device": {}, "expires_at": {}, "experiment": {},
	"experiment_id": {}, "fbclid": {}, "first_captured_at": {},
	"first_landing_path": {}, "gad_campaignid": {}, "gad_source": {},
	"gbraid": {}, "gclid": {}, "hsa_kw": {}, "hsa_mt": {}, "landing_path": {},
	"language": {}, "lng": {}, "loc_physical_ms": {}, "match_type": {},
	"medium": {}, "msclkid": {}, "network": {}, "placement": {},
	"referrer": {}, "rule_version": {}, "source": {}, "source_type": {},
	"target_id": {}, "ttclid": {}, "utm_campaign": {}, "utm_content": {},
	"utm_medium": {}, "utm_source": {}, "utm_term": {}, "wbraid": {},
	"yclid": {},
}

func isAllowedAdsAttributionKey(key string) bool {
	if _, ok := allowedAdsAttributionKeys[key]; ok {
		return true
	}
	return strings.HasPrefix(key, "utm_") || strings.HasPrefix(key, "hsa_")
}

// sanitizeAdsAttribution accepts only the small, string-valued attribution
// envelope used by the frontend. It bounds both the complete JSON document and
// every value before the data is persisted in the user row.
func sanitizeAdsAttribution(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxAdsAttributionLength {
		return ""
	}
	var payload map[string]any
	if err := common.UnmarshalJsonStr(raw, &payload); err != nil {
		return ""
	}
	cleaned := make(map[string]string, len(payload))
	for key, value := range payload {
		key = strings.TrimSpace(key)
		if key == "" || len(key) > 64 || !isAllowedAdsAttributionKey(key) {
			continue
		}
		stringValue, ok := value.(string)
		if !ok {
			continue
		}
		stringValue = strings.TrimSpace(stringValue)
		if stringValue == "" {
			continue
		}
		if len(stringValue) > 512 {
			stringValue = stringValue[:512]
		}
		cleaned[key] = stringValue
	}
	if len(cleaned) == 0 {
		return ""
	}
	encoded, err := common.Marshal(cleaned)
	if err != nil {
		return ""
	}
	return string(encoded)
}
