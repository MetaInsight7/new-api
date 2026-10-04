package model

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

func TestOpsExternalAPIKeyPredicateExcludesPlayground(t *testing.T) {
	if !strings.Contains(opsExternalAPIKeyLogPredicate, "token_id > 0") {
		t.Fatal("external API usage must require a real token")
	}
	if !strings.Contains(opsExternalAPIKeyLogPredicate, "NOT LIKE 'playground%'") {
		t.Fatal("external API usage must exclude Playground requests")
	}
}

func TestGetOpsUsersExcludesSoftDeletedAccounts(t *testing.T) {
	truncateTables(t)

	users := []*User{
		{Id: 1001, Username: "ops-enabled", Password: "password", Role: common.RoleCommonUser, AffCode: "ops1001"},
		{Id: 1002, Username: "ops-disabled", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusDisabled, AffCode: "ops1002"},
		{Id: 1003, Username: "ops-deleted", Password: "password", Role: common.RoleCommonUser, AffCode: "ops1003", DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}
	for _, user := range users {
		if err := DB.Create(user).Error; err != nil {
			t.Fatalf("create user %d: %v", user.Id, err)
		}
	}

	rows, err := GetOpsUsers(true)
	if err != nil {
		t.Fatalf("get ops users: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("ops users = %d, want enabled and disabled but not soft-deleted", len(rows))
	}
}

func TestGetOpsUserSubscriptionRemainingSumsActiveEntitlements(t *testing.T) {
	truncateTables(t)

	user := &User{Id: 1004, Username: "ops-subscriber", Password: "password", Role: common.RoleCommonUser}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	now := common.GetTimestamp()
	for _, subscription := range []*UserSubscription{
		{UserId: user.Id, AmountTotal: 1000, AmountUsed: 100, Status: "active", EndTime: now + 3600},
		{UserId: user.Id, AmountTotal: 500, AmountUsed: 600, Status: "active", EndTime: now + 7200},
		{UserId: user.Id, AmountTotal: 900, AmountUsed: 0, Status: "expired", EndTime: now + 7200},
	} {
		if err := DB.Create(subscription).Error; err != nil {
			t.Fatalf("create subscription: %v", err)
		}
	}

	remaining, err := GetOpsUserSubscriptionRemaining([]int{user.Id})
	if err != nil {
		t.Fatalf("get subscription remaining: %v", err)
	}
	if got := remaining[user.Id]; got != 900 {
		t.Fatalf("remaining quota = %d, want 900", got)
	}
}
