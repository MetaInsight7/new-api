package controller

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestInvalidateOpsReportCacheDropsReportAndCohortCaches(t *testing.T) {
	previousReport := opsReportCache
	previousReportAt := opsReportAggsAt
	previousEnabled := opsAggsCacheEnabled
	previousAll := opsAggsCacheAll
	previousAggsAt := opsAggsCacheAt
	t.Cleanup(func() {
		opsReportCache = previousReport
		opsReportAggsAt = previousReportAt
		opsAggsCacheEnabled = previousEnabled
		opsAggsCacheAll = previousAll
		opsAggsCacheAt = previousAggsAt
	})

	opsReportCache = &opsReportData{}
	opsReportAggsAt = time.Now()
	opsAggsCacheEnabled = map[int]*opsUserAgg{1: {}}
	opsAggsCacheAll = map[int]*opsUserAgg{2: {}}
	opsAggsCacheAt = time.Now()

	invalidateOpsReportCache()

	if opsReportCache != nil || !opsReportAggsAt.IsZero() {
		t.Fatalf("report cache was not invalidated: cache=%v at=%v", opsReportCache, opsReportAggsAt)
	}
	if opsAggsCacheEnabled != nil || opsAggsCacheAll != nil || !opsAggsCacheAt.IsZero() {
		t.Fatalf("cohort caches were not invalidated: enabled=%v all=%v at=%v", opsAggsCacheEnabled, opsAggsCacheAll, opsAggsCacheAt)
	}
}

func TestParseOpsAttributionPrefersFirstValidLanding(t *testing.T) {
	agg := &opsUserAgg{user: &model.OpsUser{AdsAttribution: `{"utm_campaign":"spring","first_landing_path":"/spring","landing_path":"/oauth/google"}`}}
	parseOpsAttribution(agg)
	if agg.landing != "/spring" {
		t.Fatalf("landing = %q, want /spring", agg.landing)
	}

	agg = &opsUserAgg{user: &model.OpsUser{AdsAttribution: `{"landing_path":"/oauth/google"}`}}
	parseOpsAttribution(agg)
	if agg.landing != "" {
		t.Fatalf("OAuth callback must not be reported as a landing page, got %q", agg.landing)
	}
}

func TestOpsSubscriptionOrdersAsTopUpsPreservesSource(t *testing.T) {
	orders := []*model.OpsSubscriptionOrder{
		{UserId: 1, Money: 30, Status: common.TopUpStatusSuccess, CreateTime: 100, PaymentProvider: model.PaymentProviderStripe, PlanUSD: 30},
	}
	converted := opsSubscriptionOrdersAsTopUps(orders)
	if len(converted) != 1 {
		t.Fatalf("converted rows = %d, want 1", len(converted))
	}
	row := converted[0]
	if row.Source != "subscription" || row.Money != 30 || row.PaymentProvider != model.PaymentProviderStripe {
		t.Fatalf("converted subscription = %+v", row)
	}
	if value, ok := opsTopUpUSD(row); !ok || value != 30 {
		t.Fatalf("subscription amount = %v, ok=%v; want 30", value, ok)
	}
}

func TestOpsRegisteredUsersPaidOnlyFiltersNonPayers(t *testing.T) {
	aggs := map[int]*opsUserAgg{
		1: {
			user:       &model.OpsUser{Id: 1, CreatedAt: 1, Username: "payer"},
			paidOrders: []*model.OpsTopUp{{UserId: 1, Money: 10, Status: common.TopUpStatusSuccess, CreateTime: 10}},
		},
		2: {user: &model.OpsUser{Id: 2, CreatedAt: 2, Username: "free"}},
	}
	rows := opsRegisteredUsers(aggs, map[int]int64{}, map[int]model.OpsUserSubscriptionSource{}, true)
	if len(rows) != 1 || rows[0].UserId != 1 {
		t.Fatalf("paid-only rows = %+v, want only payer", rows)
	}
}

func TestOpsSubscriptionOriginLabel(t *testing.T) {
	cases := map[string]string{
		opsSubscriptionOriginLabel("admin", ""):   "gift",
		opsSubscriptionOriginLabel("balance", ""): "wallet",
		opsSubscriptionOriginLabel("stripe", ""):  "cash",
		opsSubscriptionOriginLabel("", ""):        "-",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("origin label = %q, want %q", got, want)
		}
	}
}

func TestOpsVisibleQuotaUSDPrefersSubscriptionRemaining(t *testing.T) {
	a := &opsUserAgg{user: &model.OpsUser{Id: 11, Quota: 8000}}
	remaining := map[int]int64{11: 2500}
	if got := opsVisibleQuotaUSD(a, remaining); got != 2500.0/common.QuotaPerUnit {
		t.Fatalf("subscription balance = %v, want subscription remaining", got)
	}
}

func TestOpsUserCountryReturnsEmptyWhenBaseHasNoLocationData(t *testing.T) {
	if got := opsUserCountry(&model.OpsUser{}); got != "" {
		t.Fatalf("country = %q, want empty when no location data is available", got)
	}
}
