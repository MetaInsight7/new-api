package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestUTCDateBounds(t *testing.T) {
	start, end, err := utcDateBounds("2025-08-18")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantStart := time.Date(2025, 8, 18, 0, 0, 0, 0, time.UTC).Unix()
	if start != wantStart {
		t.Errorf("start = %d, want %d", start, wantStart)
	}
	if end-start != 24*60*60 {
		t.Errorf("day length = %d, want 86400", end-start)
	}
	if _, _, err := utcDateBounds("not-a-date"); err == nil {
		t.Error("expected error for invalid date, got nil")
	}
}

func TestUTCToday(t *testing.T) {
	now := time.Date(2025, 8, 18, 15, 4, 5, 0, time.UTC)
	if got := utcToday(now); got != "2025-08-18" {
		t.Errorf("utcToday = %q, want 2025-08-18", got)
	}
	// A near-midnight instant still maps to its UTC calendar day.
	late := time.Date(2025, 8, 18, 23, 59, 59, 0, time.UTC)
	if got := utcToday(late); got != "2025-08-18" {
		t.Errorf("utcToday(late) = %q, want 2025-08-18", got)
	}
}

func TestAggregateUsageReportDateUsesBaseSchema(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get test db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.Token{}, &model.TopUp{}, &model.Log{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

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
	if err != nil {
		t.Fatalf("date bounds: %v", err)
	}
	users := []*model.User{
		{Id: 2001, Username: "usage-active", Password: "password", AffCode: "usage-active-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, CreatedAt: start + 3600},
		{Id: 2002, Username: "usage-free", Password: "password", AffCode: "usage-free-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, CreatedAt: start + 7200},
		{Id: 2003, Username: "usage-admin", Password: "password", AffCode: "usage-admin-aff", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, CreatedAt: start + 3600},
	}
	for _, user := range users {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("create user %d: %v", user.Id, err)
		}
	}
	if err := db.Create(&model.Token{
		Id:          2101,
		UserId:      2001,
		Key:         "usage-token",
		Name:        "manual",
		CreatedTime: start + 7200,
	}).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	if err := db.Create(&model.TopUp{
		Id:           2201,
		UserId:       2001,
		Money:        12.5,
		TradeNo:      "usage-topup",
		Status:       common.TopUpStatusSuccess,
		CreateTime:   start + 9000,
		CompleteTime: start + 9000,
	}).Error; err != nil {
		t.Fatalf("create top-up: %v", err)
	}
	for _, log := range []*model.Log{
		{Id: 2301, UserId: 2001, CreatedAt: start + 10000, Type: model.LogTypeConsume, ModelName: "gpt-test", PromptTokens: 100, CompletionTokens: 40},
		{Id: 2302, UserId: 2001, CreatedAt: end - 1, Type: model.LogTypeConsume, ModelName: "gpt-test", PromptTokens: 20, CompletionTokens: 10},
	} {
		if err := db.Create(log).Error; err != nil {
			t.Fatalf("create log: %v", err)
		}
	}

	day, models, err := aggregateUsageReportDate(date, start, end)
	if err != nil {
		t.Fatalf("aggregate usage report: %v", err)
	}
	if day.Registered != 2 {
		t.Fatalf("registered = %d, want 2 ordinary enabled users", day.Registered)
	}
	if day.ActivatedKey != 1 || day.FirstPaid != 1 {
		t.Fatalf("first events = (%d, %d), want (1, 1)", day.ActivatedKey, day.FirstPaid)
	}
	if day.ActivatedDay != 1 || day.PaidDay != 1 {
		t.Fatalf("same-day funnel = (%d, %d), want (1, 1)", day.ActivatedDay, day.PaidDay)
	}
	if day.PaidUSD != 12.5 || day.Calls != 2 || day.PromptTokens != 120 || day.CompletionTokens != 50 {
		t.Fatalf("day totals = %+v", day)
	}
	if len(models) != 1 || models[0].ModelName != "gpt-test" || models[0].Calls != 2 {
		t.Fatalf("model rows = %+v", models)
	}
}
