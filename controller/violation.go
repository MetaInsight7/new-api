/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

For commercial licensing, please contact support@quantumnous.com
*/

// Package controller — 违规审计接口(仅管理员只读 + 词库管理)。
// 数据落 LOG_DB 的 violation_logs 表(见 model/violation.go)。
package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

// GetViolationLogs 违规审计记录分页查询。
func GetViolationLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	modelName := c.Query("model_name")
	category := c.Query("category")
	severity := c.Query("severity")
	action := c.Query("action")
	requestId := c.Query("request_id")

	logs, total, err := model.GetViolationLogs(
		startTimestamp, endTimestamp, username, modelName, category, severity, action, requestId,
		pageInfo.GetStartIdx(), pageInfo.GetPageSize(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

// GetViolationStat 违规监控聚合(趋势/拦截占比/Top用户/Top词/分类占比)。
func GetViolationStat(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	stat, err := model.GetViolationStat(startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stat)
}

// GetViolationWords 获取违规词库(分类分级)。
func GetViolationWords(c *gin.Context) {
	common.ApiSuccess(c, gin.H{
		"words":      setting.GetViolationWords(),
		"categories": setting.ViolationCategories,
		"severities": setting.ViolationSeverities,
		"enabled":    setting.ViolationAuditEnabled,
	})
}

type updateViolationWordsReq struct {
	Words []setting.ViolationWord `json:"words"`
}

// UpdateViolationWords 保存违规词库(持久化到 options,即时重建 AC)。
func UpdateViolationWords(c *gin.Context) {
	var req updateViolationWordsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	b, err := common.Marshal(req.Words)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// UpdateOption 会写库并触发 updateOptionMap -> setting.SetViolationWords 重建索引
	if err := model.UpdateOption("ViolationWordLibrary", string(b)); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"words": setting.GetViolationWords()})
}
