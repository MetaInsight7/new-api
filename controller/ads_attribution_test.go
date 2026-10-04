package controller

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeAdsAttribution(t *testing.T) {
	longValue := strings.Repeat("x", 700)
	raw := `{"utm_campaign":" spring ","gclid":"click-1","aff":"partner","nested":{"secret":"no"},"count":4,"unknown":"drop","utm_content":"` + longValue + `"}`
	cleaned := sanitizeAdsAttribution(raw)
	if cleaned == "" {
		t.Fatal("sanitizeAdsAttribution returned empty output")
	}

	var values map[string]string
	if err := json.Unmarshal([]byte(cleaned), &values); err != nil {
		t.Fatalf("cleaned attribution is not JSON: %v", err)
	}
	if values["utm_campaign"] != "spring" || values["gclid"] != "click-1" || values["aff"] != "partner" {
		t.Fatalf("allowed fields were not preserved: %+v", values)
	}
	if _, ok := values["nested"]; ok {
		t.Fatal("nested attribution value was persisted")
	}
	if _, ok := values["count"]; ok {
		t.Fatal("non-string attribution value was persisted")
	}
	if _, ok := values["unknown"]; ok {
		t.Fatal("unknown attribution key was persisted")
	}
	if len(values["utm_content"]) != 512 {
		t.Fatalf("long attribution value length = %d, want 512", len(values["utm_content"]))
	}
}

func TestSanitizeAdsAttributionRejectsOversizedDocument(t *testing.T) {
	if got := sanitizeAdsAttribution(`{"utm_source":"` + strings.Repeat("x", maxAdsAttributionLength) + `"}`); got != "" {
		t.Fatal("oversized attribution document was accepted")
	}
}

func TestPrimaryAcceptLanguage(t *testing.T) {
	if got := primaryAcceptLanguage("zh-CN,zh;q=0.9,en;q=0.8"); got != "zh-CN" {
		t.Fatalf("primary language = %q, want zh-CN", got)
	}
	if got := primaryAcceptLanguage(" en-US ;q=0.8 "); got != "en-US" {
		t.Fatalf("weighted language = %q, want en-US", got)
	}
	if got := primaryAcceptLanguage(""); got != "" {
		t.Fatalf("empty language = %q, want empty", got)
	}
}
