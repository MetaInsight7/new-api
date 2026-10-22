package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gorm.io/gorm"
)

func TestOpsExternalAPIKeyPredicateExcludesPlayground(t *testing.T) {
	assert.Contains(t, opsExternalAPIKeyLogPredicate, "token_id > 0")
	assert.Contains(t, opsExternalAPIKeyLogPredicate, "NOT LIKE 'playground%'")
}

func TestGetOpsUsersExcludesSoftDeletedAccounts(t *testing.T) {
	truncateTables(t)

	users := []*User{
		{Id: 1001, Username: "ops-enabled", Password: "password", Role: common.RoleCommonUser, AffCode: "ops1001"},
		{Id: 1002, Username: "ops-disabled", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusDisabled, AffCode: "ops1002"},
		{Id: 1003, Username: "ops-deleted", Password: "password", Role: common.RoleCommonUser, AffCode: "ops1003", DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true}},
	}
	for _, user := range users {
		require.NoError(t, DB.Create(user).Error, "create user %d", user.Id)
	}

	rows, err := GetOpsUsers(true)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

func TestGetOpsUserSubscriptionRemainingSumsActiveEntitlements(t *testing.T) {
	truncateTables(t)

	user := &User{Id: 1004, Username: "ops-subscriber", Password: "password", Role: common.RoleCommonUser}
	require.NoError(t, DB.Create(user).Error)
	now := common.GetTimestamp()
	for _, subscription := range []*UserSubscription{
		{UserId: user.Id, AmountTotal: 1000, AmountUsed: 100, Status: "active", EndTime: now + 3600},
		{UserId: user.Id, AmountTotal: 500, AmountUsed: 600, Status: "active", EndTime: now + 7200},
		{UserId: user.Id, AmountTotal: 900, AmountUsed: 0, Status: "expired", EndTime: now + 7200},
	} {
		require.NoError(t, DB.Create(subscription).Error)
	}

	remaining, err := GetOpsUserSubscriptionRemaining([]int{user.Id})
	require.NoError(t, err)
	assert.Equal(t, int64(900), remaining[user.Id])
}
