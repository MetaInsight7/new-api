package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUTCDateBounds(t *testing.T) {
	start, end, err := utcDateBounds("2025-08-18")
	require.NoError(t, err)
	wantStart := time.Date(2025, 8, 18, 0, 0, 0, 0, time.UTC).Unix()
	assert.Equal(t, wantStart, start)
	assert.Equal(t, int64(24*60*60), end-start)
	_, _, err = utcDateBounds("not-a-date")
	assert.Error(t, err)
}

func TestUTCToday(t *testing.T) {
	now := time.Date(2025, 8, 18, 15, 4, 5, 0, time.UTC)
	assert.Equal(t, "2025-08-18", utcToday(now))
	// A near-midnight instant still maps to its UTC calendar day.
	late := time.Date(2025, 8, 18, 23, 59, 59, 0, time.UTC)
	assert.Equal(t, "2025-08-18", utcToday(late))
}

func TestAggregateUsageReportDateUsesBaseSchema(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.TopUp{}, &model.Log{}))

	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldMainType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.SetDatabaseTypes(oldMainType, oldLogType)
		_ = sqlDB.Close()
	})

	const date = "2026-10-04"
	start, end, err := utcDateBounds(date)
	require.NoError(t, err)
	users := []*model.User{
		{Id: 2001, Username: "usage-active", Password: "password", AffCode: "usage-active-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, CreatedAt: start + 3600},
		{Id: 2002, Username: "usage-free", Password: "password", AffCode: "usage-free-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, CreatedAt: start + 7200},
		{Id: 2003, Username: "usage-admin", Password: "password", AffCode: "usage-admin-aff", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, CreatedAt: start + 3600},
	}
	for _, user := range users {
		require.NoError(t, db.Create(user).Error, "create user %d", user.Id)
	}
	require.NoError(t, db.Create(&model.Token{
		Id:          2101,
		UserId:      2001,
		Key:         "usage-token",
		Name:        "manual",
		CreatedTime: start + 7200,
	}).Error)
	require.NoError(t, db.Create(&model.TopUp{
		Id:           2201,
		UserId:       2001,
		Money:        12.5,
		TradeNo:      "usage-topup",
		Status:       common.TopUpStatusSuccess,
		CreateTime:   start + 9000,
		CompleteTime: start + 9000,
	}).Error)
	for _, log := range []*model.Log{
		{Id: 2301, UserId: 2001, CreatedAt: start + 10000, Type: model.LogTypeConsume, ModelName: "gpt-test", PromptTokens: 100, CompletionTokens: 40},
		{Id: 2302, UserId: 2001, CreatedAt: end - 1, Type: model.LogTypeConsume, ModelName: "gpt-test", PromptTokens: 20, CompletionTokens: 10},
	} {
		require.NoError(t, db.Create(log).Error)
	}

	day, models, err := aggregateUsageReportDate(date, start, end)
	require.NoError(t, err)
	assert.Equal(t, 2, day.Registered)
	assert.Equal(t, 1, day.ActivatedKey)
	assert.Equal(t, 1, day.FirstPaid)
	assert.Equal(t, 1, day.ActivatedDay)
	assert.Equal(t, 1, day.PaidDay)
	assert.Equal(t, 12.5, day.PaidUSD)
	assert.Equal(t, int64(2), day.Calls)
	assert.Equal(t, int64(120), day.PromptTokens)
	assert.Equal(t, int64(50), day.CompletionTokens)
	require.Len(t, models, 1)
	assert.Equal(t, "gpt-test", models[0].ModelName)
	assert.Equal(t, int64(2), models[0].Calls)
}
