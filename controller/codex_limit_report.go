package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetCodexLimitReport(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	endTimestamp := common.GetTimestamp()
	startTimestamp := endTimestamp - 7*24*3600
	startQuery, endQuery := ctx.Query("start_timestamp"), ctx.Query("end_timestamp")
	if startQuery != "" || endQuery != "" {
		var startErr, endErr error
		startTimestamp, startErr = strconv.ParseInt(startQuery, 10, 64)
		endTimestamp, endErr = strconv.ParseInt(endQuery, 10, 64)
		if startErr != nil || endErr != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "message": service.ErrCodexLimitReportRange.Error()})
			return
		}
	}
	report, err := service.GetCodexLimitReport(ctx.Request.Context(), startTimestamp, endTimestamp)
	if err != nil {
		status := http.StatusInternalServerError
		message := "Failed to fetch Codex limits"
		if errors.Is(err, service.ErrCodexLimitReportRange) {
			status, message = http.StatusBadRequest, err.Error()
		}
		ctx.JSON(status, gin.H{"success": false, "message": message})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": report})
}
