package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting"
)

// 验证结构化提取 + 双范围:高危(任意角色/历史)拦截,低危仅按最新 user 记录。
func TestBuildAuditScan_OpenAI(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{Role: "system", Content: "you are helpful"},
			{Role: "user", Content: "old question with lowword"},
			{Role: "assistant", Content: "some answer highword here"},
			{Role: "user", Content: "new clean question"},
		},
	}
	scan := BuildAuditScan(req, nil)

	// FullText 含所有角色文本(含 assistant 里的 highword)
	if !contains(scan.FullText, "highword") || !contains(scan.FullText, "lowword") {
		t.Fatalf("full text should include all roles, got: %q", scan.FullText)
	}
	// LatestUserText 只应是最新那条 user
	if scan.LatestUserText != "new clean question" {
		t.Fatalf("latest user text wrong: %q", scan.LatestUserText)
	}
	// Context 是结构化 JSON,含 role
	if !contains(scan.ContextJSON, "\"role\"") || !contains(scan.ContextJSON, "assistant") {
		t.Fatalf("context should be structured json: %q", scan.ContextJSON)
	}
}

func TestViolation_TwoScope(t *testing.T) {
	setting.SetViolationWords(`[
		{"word":"highword","category":"politics","severity":"high"},
		{"word":"lowword","category":"abuse","severity":"low"}
	]`)
	defer setting.SetViolationWords("")

	// 场景1:高危藏在 assistant 历史里 → 全量扫应拦截(堵绕过)
	full := "you are helpful\nassistant said highword\nnew clean question"
	if r := CheckViolation(full); !r.Block {
		t.Fatalf("high word anywhere should block, got %+v", r)
	}

	// 场景2:低危在历史 user、最新 user 干净 → 只扫最新 user 不记
	latestClean := "new clean question"
	if r := CheckViolation(latestClean); r.Hit {
		t.Fatalf("clean latest user should not hit, got %+v", r)
	}

	// 场景3:低危就在最新 user → 记录(不拦)
	latestLow := "please do lowword now"
	r := CheckViolation(latestLow)
	if !r.Hit || r.Block {
		t.Fatalf("low in latest user should hit but not block, got %+v", r)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
