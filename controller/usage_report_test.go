package controller

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func TestWriteUsageReportDailyCSV(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	writeUsageReportDailyCSV(context, []*model.UsageReportDay{
		{
			Date:             "2026-10-04",
			Registered:       3,
			ActivatedDay:     2,
			PaidDay:          1,
			PaidUSD:          12.5,
			Calls:            7,
			PromptTokens:     100,
			CompletionTokens: 50,
		},
	})

	if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, "usage_report_daily.csv") {
		t.Fatalf("content disposition = %q", got)
	}
	want := "date,registered,activated_day,paid_day,paid_usd,calls,tokens\n2026-10-04,3,2,1,12.50,7,150\n"
	if recorder.Body.String() != want {
		t.Fatalf("csv = %q, want %q", recorder.Body.String(), want)
	}
}

func TestWriteUsageReportModelsCSV(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	writeUsageReportModelsCSV(context, []*model.UsageReportDayModel{
		{
			Date:             "2026-10-04",
			ModelName:        "gpt-test",
			Calls:            4,
			PromptTokens:     80,
			CompletionTokens: 20,
		},
	})

	if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, "usage_report_models.csv") {
		t.Fatalf("content disposition = %q", got)
	}
	want := "date,model,calls,tokens\n2026-10-04,gpt-test,4,100\n"
	if recorder.Body.String() != want {
		t.Fatalf("csv = %q, want %q", recorder.Body.String(), want)
	}
}
