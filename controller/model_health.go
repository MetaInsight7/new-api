package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/gin-gonic/gin"
)

func GetModelHealthOverview(c *gin.Context) {
	hours, ok := parseModelHealthHours(c)
	if !ok {
		return
	}
	result, err := perfmetrics.GetModelHealthOverview(hours)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetModelHealthDetail(c *gin.Context) {
	modelName := strings.TrimSpace(c.Query("model"))
	if modelName == "" {
		modelHealthBadRequest(c, "model is required")
		return
	}
	hours, ok := parseModelHealthHours(c)
	if !ok {
		return
	}
	result, err := perfmetrics.GetModelHealthDetail(modelName, hours)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func parseModelHealthHours(c *gin.Context) (int, bool) {
	raw := strings.TrimSpace(c.Query("hours"))
	if raw == "" {
		return 24, true
	}
	hours, err := strconv.Atoi(raw)
	if err != nil {
		modelHealthBadRequest(c, "hours must be one of 24, 168, or 720")
		return 0, false
	}
	if err = perfmetrics.ValidateModelHealthHours(hours); err != nil {
		modelHealthBadRequest(c, "hours must be one of 24, 168, or 720")
		return 0, false
	}
	return hours, true
}

func modelHealthBadRequest(c *gin.Context, messageKey string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"success": false,
		"message": messageKey,
	})
}
