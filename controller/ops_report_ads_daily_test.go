package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	adsTestCampaignPrefix = "brand"
	adsTestLandingHost    = "brand.example"
)

func adsTestKw(date, adGroup, criterion, keyword string, snapshot bool, bid, cost float64, clicks int, status string) *model.AdsDailyKeyword {
	return &model.AdsDailyKeyword{
		Date: date, AdGroupId: adGroup, CriterionId: criterion,
		CampaignName: adsTestCampaignPrefix + "-EN-Search", AdGroupName: "ag", Keyword: keyword,
		MatchType: "PHRASE", Status: status, CpcBidUSD: bid, Snapshot: snapshot,
		CostUSD: cost, Clicks: clicks,
	}
}

func adsTestAd(date, adId string, snapshot bool, headlines string, cost float64) *model.AdsDailyCreative {
	return &model.AdsDailyCreative{
		Date: date, AdId: adId, CampaignName: adsTestCampaignPrefix + "-EN-Search", AdGroupName: "ag",
		AdType: "RESPONSIVE_SEARCH_AD", Status: "ENABLED", Snapshot: snapshot,
		Headlines: headlines, Descriptions: `["d1"]`, ImageUrls: "[]",
		FinalUrls: `["https://brand.example/x"]`, CostUSD: cost,
	}
}

func adsDailyFindDay(t *testing.T, days []*adsDailyDay, date string) *adsDailyDay {
	t.Helper()
	for _, d := range days {
		if d.Date == date {
			return d
		}
	}
	require.FailNow(t, "day not found", "date: %s", date)
	return nil
}

func TestBuildAdsDailyDaysDiffs(t *testing.T) {
	keywords := []*model.AdsDailyKeyword{
		// day 1: first snapshot — no changes expected (no previous snapshot)
		adsTestKw("2026-07-18", "g1", "k1", "claude api", true, 0.5, 1.2, 3, "ENABLED"),
		adsTestKw("2026-07-18", "g1", "k2", "gpt api", true, 0.8, 2.0, 4, "ENABLED"),
		// day 2: k1 bid raised, k2 gone from snapshot (still has metrics), k3 added
		adsTestKw("2026-07-19", "g1", "k1", "claude api", true, 0.7, 1.5, 2, "ENABLED"),
		adsTestKw("2026-07-19", "g1", "k2", "gpt api", false, 0, 0.4, 1, ""),
		adsTestKw("2026-07-19", "g1", "k3", "gemini api", true, 0.6, 0, 0, "PAUSED"),
	}
	creatives := []*model.AdsDailyCreative{
		adsTestAd("2026-07-18", "a1", true, `["h1","h2"]`, 3.0),
		adsTestAd("2026-07-19", "a1", true, `["h1","h2-edited"]`, 2.5),
	}
	days := buildAdsDailyDays(keywords, creatives, nil)
	require.Len(t, days, 2)
	assert.Equal(t, "2026-07-19", days[0].Date)

	d1 := adsDailyFindDay(t, days, "2026-07-18")
	assert.True(t, d1.Snapshot)
	assert.Equal(t, adsDailyChangeSummary{}, d1.Changes)
	// totals sum the per-ad metrics
	assert.Equal(t, 3.0, d1.CostUSD)

	d2 := adsDailyFindDay(t, days, "2026-07-19")
	want := adsDailyChangeSummary{KeywordsAdded: 1, KeywordsRemoved: 1, BidChanges: 1, CreativeChanges: 1}
	assert.Equal(t, want, d2.Changes)
	byKw := map[string]adsDailyKeywordRow{}
	for _, r := range d2.Keywords {
		byKw[r.Keyword] = r
	}
	assert.Equal(t, "bid_changed", byKw["claude api"].Change)
	assert.Equal(t, 0.5, byKw["claude api"].PrevBidUSD)
	assert.Equal(t, 0.7, byKw["claude api"].CpcBidUSD)
	assert.Equal(t, "removed", byKw["gpt api"].Change)
	assert.Equal(t, 1, byKw["gpt api"].Clicks)
	assert.Equal(t, "added", byKw["gemini api"].Change)
	assert.Equal(t, "PAUSED", byKw["gemini api"].Status)
	require.Len(t, d2.Creatives, 1)
	assert.Equal(t, "content_changed", d2.Creatives[0].Change)
}

func TestBuildAdsDailyDaysMetricsOnlyHistory(t *testing.T) {
	// pre-feature history: metrics rows exist but no snapshots — no changes,
	// and days render with metrics only.
	keywords := []*model.AdsDailyKeyword{
		adsTestKw("2026-07-01", "g1", "k1", "claude api", false, 0, 1.0, 2, ""),
		adsTestKw("2026-07-02", "g1", "k1", "claude api", false, 0, 2.0, 3, ""),
	}
	days := buildAdsDailyDays(keywords, nil, nil)
	require.Len(t, days, 2)
	for _, d := range days {
		assert.False(t, d.Snapshot, d.Date)
		assert.Equal(t, adsDailyChangeSummary{}, d.Changes, d.Date)
	}
}

func TestAdsDailyConfiguredScopeFilter(t *testing.T) {
	kws := adsDailyFilterKeywords([]*model.AdsDailyKeyword{
		{CampaignName: "brand-PT-Search", Keyword: "kimi 3.0"},
		{CampaignName: "VOC-API-MCP-Dev-US", Keyword: "amazon review api"},
		{CampaignName: "solvea-USCA-Search-Verticals", Keyword: "ai receptionist"},
		{CampaignName: "brandkeyboard-US", Keyword: "mechanical keyboard"},
	}, adsTestCampaignPrefix)
	require.Len(t, kws, 1)
	assert.Equal(t, "kimi 3.0", kws[0].Keyword)
	ads := adsDailyFilterCreatives([]*model.AdsDailyCreative{
		{CampaignName: "brand-SEA-Search", AdId: "a1"},
		{CampaignName: "Android US App Installs - AI Receptionist Translation", AdId: "a2"},
	}, adsTestCampaignPrefix)
	require.Len(t, ads, 1)
	assert.Equal(t, "a1", ads[0].AdId)
	lps := adsDailyFilterLandings([]*model.AdsDailyLanding{
		{Url: "https://brand.example/pt/models/gemini-api"},
		{Url: "https://www.brand.example/chinese-ai"},
		{Url: "https://www.voc.ai/api/amazon-data"},
		{Url: "https://solvea.cx/receptionist"},
		{Url: "https://evil.brand.example/phish"},
		{Url: "https://not-brand.example/path?next=brand.example"},
		{Url: "https://evil.example/?u=https://brand.example"},
		{Url: "javascript://brand.example/%0aalert(1)"},
		{Url: "file://brand.example/etc/passwd"},
		{Url: "//brand.example/scheme-relative"},
	}, adsTestLandingHost)
	require.Len(t, lps, 2)
	assert.Equal(t, "https://brand.example/pt/models/gemini-api", lps[0].Url)
	assert.Equal(t, "https://www.brand.example/chinese-ai", lps[1].Url)
}

func TestBuildAdsDailyLandingsDiff(t *testing.T) {
	curAds := map[string]*model.AdsDailyCreative{
		"a1": {Snapshot: true, FinalUrls: `["https://brand.example/new"]`},
	}
	prevAds := map[string]*model.AdsDailyCreative{
		"a1": {Snapshot: true, FinalUrls: `["https://brand.example/old"]`},
	}
	day := &adsDailyDay{Snapshot: true}
	rows := buildAdsDailyLandings([]*model.AdsDailyLanding{
		{Url: "https://brand.example/new", Clicks: 5, CostUSD: 2.5},
	}, curAds, prevAds, day)
	byUrl := map[string]adsDailyLandingRow{}
	for _, r := range rows {
		byUrl[r.Url] = r
	}
	assert.Equal(t, "added", byUrl["https://brand.example/new"].Change)
	assert.Equal(t, 5, byUrl["https://brand.example/new"].Clicks)
	assert.Equal(t, "removed", byUrl["https://brand.example/old"].Change)
	assert.Equal(t, 2, day.Changes.LandingChanges)
}

func TestAdsThumbTarget(t *testing.T) {
	got := adsThumbTarget("https://brand.example/sign-up?lng=pt&a=b#frag one")
	want := "https://brand.example/sign-up%3Flng%3Dpt%26a%3Db%23frag%20one"
	assert.Equal(t, want, got)
}
