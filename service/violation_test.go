package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
)

// 违规检测核心逻辑测试:高危拦截、低危放行、分类映射、词库为空回退。
func TestCheckViolation_SeverityAndCategory(t *testing.T) {
	// 设置分类分级词库
	setting.SetViolationWords(`[
		{"word":"highword","category":"politics","severity":"high"},
		{"word":"lowword","category":"abuse","severity":"low"}
	]`)
	defer setting.SetViolationWords("") // 复原,避免影响其它测试

	// 高危命中 => Hit 且 Block
	r := CheckViolation("hello HIGHWORD world")
	if !r.Hit || !r.Block {
		t.Fatalf("high word should hit+block, got hit=%v block=%v", r.Hit, r.Block)
	}
	if r.Severity != "high" {
		t.Fatalf("expected severity high, got %s", r.Severity)
	}
	if len(r.Categories) != 1 || r.Categories[0] != "politics" {
		t.Fatalf("expected category politics, got %v", r.Categories)
	}

	// 低危命中 => Hit 但不 Block
	r = CheckViolation("this contains lowword only")
	if !r.Hit || r.Block {
		t.Fatalf("low word should hit but not block, got hit=%v block=%v", r.Hit, r.Block)
	}
	if r.Severity != "low" {
		t.Fatalf("expected severity low, got %s", r.Severity)
	}

	// 未命中
	if r := CheckViolation("perfectly clean text"); r.Hit {
		t.Fatalf("clean text should not hit, got %+v", r)
	}

	// 同时命中高低 => Block(高危优先)
	r = CheckViolation("highword and lowword together")
	if !r.Block || r.Severity != "high" {
		t.Fatalf("mixed hit should block with high severity, got block=%v sev=%s", r.Block, r.Severity)
	}
}
