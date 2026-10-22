package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOpsAttributionPrefersFirstValidLanding(t *testing.T) {
	agg := &opsUserAgg{user: &model.OpsUser{AdsAttribution: `{"utm_campaign":"spring","first_landing_path":"/spring","landing_path":"/oauth/google"}`}}
	parseOpsAttribution(agg)
	assert.Equal(t, "/spring", agg.landing)

	agg = &opsUserAgg{user: &model.OpsUser{AdsAttribution: `{"landing_path":"/oauth/google"}`}}
	parseOpsAttribution(agg)
	assert.Empty(t, agg.landing)
}

func TestOpsSubscriptionOrdersAsTopUpsPreservesSource(t *testing.T) {
	orders := []*model.OpsSubscriptionOrder{
		{UserId: 1, Money: 30, Status: common.TopUpStatusSuccess, CreateTime: 100, PaymentProvider: model.PaymentProviderStripe, PlanUSD: 30},
	}
	converted := opsSubscriptionOrdersAsTopUps(orders)
	require.Len(t, converted, 1)
	row := converted[0]
	assert.Equal(t, "subscription", row.Source)
	assert.Equal(t, 30.0, row.Money)
	assert.Equal(t, model.PaymentProviderStripe, row.PaymentProvider)
	value, ok := opsTopUpUSD(row)
	require.True(t, ok)
	assert.Equal(t, 30.0, value)
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
	require.Len(t, rows, 1)
	assert.Equal(t, 1, rows[0].UserId)
}

func TestOpsSubscriptionOriginLabel(t *testing.T) {
	cases := map[string]string{
		opsSubscriptionOriginLabel("admin", ""):   "gift",
		opsSubscriptionOriginLabel("balance", ""): "wallet",
		opsSubscriptionOriginLabel("stripe", ""):  "cash",
		opsSubscriptionOriginLabel("", ""):        "-",
	}
	for got, want := range cases {
		assert.Equal(t, want, got)
	}
}

func TestOpsVisibleQuotaUSDPrefersSubscriptionRemaining(t *testing.T) {
	a := &opsUserAgg{user: &model.OpsUser{Id: 11, Quota: 8000}}
	remaining := map[int]int64{11: 2500}
	assert.Equal(t, 2500.0/common.QuotaPerUnit, opsVisibleQuotaUSD(a, remaining))
}

func TestOpsUserCountryReturnsEmptyWhenBaseHasNoLocationData(t *testing.T) {
	assert.Empty(t, opsUserCountry(&model.OpsUser{}))
}

func TestOpsStripeLocationUsesUserIdentitySignals(t *testing.T) {
	row := opsStripePersonRow{LastIP: "203.0.113.20", IPCountry: "SG"}
	user := &model.OpsUser{Id: 1, PayCountry: "US", LastLoginIp: "198.51.100.10"}

	opsStripeApplyUserLocation(&row, user)

	assert.Equal(t, user.LastLoginIp, row.LastIP)
	assert.Equal(t, "US", row.IPCountry)
}

func TestOpsStripeLocationDoesNotUseAPIRequestIPFallback(t *testing.T) {
	row := opsStripePersonRow{LastIP: "203.0.113.20", IPCountry: "SG"}
	opsStripeApplyUserLocation(&row, &model.OpsUser{Id: 1})

	assert.Empty(t, row.LastIP)
	assert.Empty(t, row.IPCountry)
}
