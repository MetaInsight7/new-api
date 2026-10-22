package controller

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeAdsAttribution(t *testing.T) {
	longValue := strings.Repeat("x", 700)
	raw := `{"utm_campaign":" spring ","gclid":"click-1","aff":"partner","nested":{"secret":"no"},"count":4,"unknown":"drop","utm_content":"` + longValue + `"}`
	cleaned := sanitizeAdsAttribution(raw)
	require.NotEmpty(t, cleaned)

	var values map[string]string
	require.NoError(t, json.Unmarshal([]byte(cleaned), &values))
	assert.Equal(t, "spring", values["utm_campaign"])
	assert.Equal(t, "click-1", values["gclid"])
	assert.Equal(t, "partner", values["aff"])
	assert.NotContains(t, values, "nested")
	assert.NotContains(t, values, "count")
	assert.NotContains(t, values, "unknown")
	assert.Len(t, values["utm_content"], 512)
}

func TestSanitizeAdsAttributionRejectsOversizedDocument(t *testing.T) {
	assert.Empty(t, sanitizeAdsAttribution(`{"utm_source":"`+strings.Repeat("x", maxAdsAttributionLength)+`"}`))
}

func TestPrimaryAcceptLanguage(t *testing.T) {
	assert.Equal(t, "zh-CN", primaryAcceptLanguage("zh-CN,zh;q=0.9,en;q=0.8"))
	assert.Equal(t, "en-US", primaryAcceptLanguage(" en-US ;q=0.8 "))
	assert.Empty(t, primaryAcceptLanguage(""))
}
