package controller

import (
	"cmp"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	channelLoadMonitorDefaultPageSize = 20
	channelLoadMonitorMaxPageSize     = 100
)

type channelLoadMonitorItem struct {
	Channel     *model.Channel            `json:"channel"`
	Concurrency *channelConcurrencyStatus `json:"concurrency"`
}

type channelLoadMonitorSummary struct {
	LimitedRPM  int64 `json:"limited_rpm"`
	Active      int   `json:"active"`
	Capacity    int   `json:"capacity"`
	Waiting     int   `json:"waiting"`
	CoolingDown int   `json:"cooling_down"`
}

type channelLoadMonitorRecord struct {
	channel *model.Channel
	load    service.ChannelConcurrencyLoad
}

func GetChannelLoadMonitor(c *gin.Context) {
	page, pageSize := channelLoadMonitorPagination(c)
	groupFilter := model.NormalizeChannelGroupFilter(c.Query("group"))
	statusFilter := parseStatusFilter(c.Query("status"))
	typeFilter := -1
	if rawType := strings.TrimSpace(c.Query("type")); rawType != "" {
		if parsedType, err := strconv.Atoi(rawType); err == nil {
			typeFilter = parsedType
		}
	}

	groupValues := make([]string, 0)
	if err := model.DB.Model(&model.Channel{}).
		Where("max_concurrency > 0").
		Distinct().
		Pluck("group", &groupValues).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	groupSet := make(map[string]struct{})
	for _, groupValue := range groupValues {
		for _, group := range strings.Split(strings.Trim(groupValue, ","), ",") {
			group = strings.TrimSpace(group)
			if group != "" {
				groupSet[group] = struct{}{}
			}
		}
	}
	groups := make([]string, 0, len(groupSet))
	for group := range groupSet {
		groups = append(groups, group)
	}
	sort.Strings(groups)

	query := buildChannelListQuery(groupFilter, statusFilter, -1).
		Where("max_concurrency > 0").
		Omit("key")
	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword != "" {
		namePattern := "%" + keyword + "%"
		if channelID, err := strconv.Atoi(keyword); err == nil && channelID > 0 {
			query = query.Where("id = ? OR name LIKE ?", channelID, namePattern)
		} else {
			query = query.Where("name LIKE ?", namePattern)
		}
	}

	channels := make([]*model.Channel, 0)
	if err := query.Find(&channels).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	typeCounts := make(map[int]int64)
	filteredChannels := make([]*model.Channel, 0, len(channels))
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		typeCounts[channel.Type]++
		if typeFilter < 0 || channel.Type == typeFilter {
			filteredChannels = append(filteredChannels, channel)
		}
	}

	loads, err := service.GetChannelConcurrencyLoads(c.Request.Context(), filteredChannels)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	loadState := strings.ToLower(strings.TrimSpace(c.Query("load_state")))
	records := make([]channelLoadMonitorRecord, 0, len(filteredChannels))
	for _, channel := range filteredChannels {
		load := loads[channel.Id]
		if !channelLoadMonitorMatchesState(channel, load, loadState) {
			continue
		}
		records = append(records, channelLoadMonitorRecord{channel: channel, load: load})
	}

	channelLoadMonitorSort(records, c.Query("sort_by"), c.Query("sort_order"))

	snapshotAt := time.Now()
	channelRPMs := map[int]int64{}
	if len(records) > 0 {
		channelRPMs, err = model.GetChannelRPMsSince(c.Request.Context(), snapshotAt.Add(-time.Minute).Unix())
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}

	summary := channelLoadMonitorSummary{}
	for _, record := range records {
		summary.LimitedRPM += channelRPMs[record.channel.Id]
		summary.Active += record.load.Active
		summary.Capacity += record.channel.GetMaxConcurrency()
		summary.Waiting += record.load.Waiting
		if record.load.CoolingDown {
			summary.CoolingDown++
		}
	}

	total := len(records)
	start := min((page-1)*pageSize, total)
	end := min(start+pageSize, total)
	items := make([]channelLoadMonitorItem, 0, end-start)
	for _, record := range records[start:end] {
		clearChannelInfo(record.channel)
		status := channelConcurrencyStatus{
			ChannelID:      record.channel.Id,
			MaxConcurrency: record.channel.GetMaxConcurrency(),
			Active:         record.load.Active,
			Waiting:        record.load.Waiting,
			CoolingDown:    record.load.CoolingDown,
		}
		item := channelLoadMonitorItem{Channel: record.channel, Concurrency: &status}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items":       items,
			"total":       total,
			"page":        page,
			"page_size":   pageSize,
			"type_counts": typeCounts,
			"groups":      groups,
			"summary":     summary,
			"snapshot_at": snapshotAt.UnixMilli(),
		},
	})
}

func channelLoadMonitorPagination(c *gin.Context) (int, int) {
	page, err := strconv.Atoi(c.Query("p"))
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(c.Query("page_size"))
	if err != nil || pageSize < 1 {
		pageSize = channelLoadMonitorDefaultPageSize
	}
	if pageSize > channelLoadMonitorMaxPageSize {
		pageSize = channelLoadMonitorMaxPageSize
	}
	return page, pageSize
}

func channelLoadMonitorMatchesState(channel *model.Channel, load service.ChannelConcurrencyLoad, state string) bool {
	switch state {
	case "", "all":
		return true
	case "full":
		return channel.GetMaxConcurrency() > 0 && load.Active >= channel.GetMaxConcurrency()
	case "queued":
		return channel.GetMaxConcurrency() > 0 && load.Waiting > 0
	case "cooling":
		return channel.GetMaxConcurrency() > 0 && load.CoolingDown
	case "unlimited":
		return false
	default:
		return true
	}
}

func channelLoadMonitorSort(records []channelLoadMonitorRecord, rawSortBy string, rawSortOrder string) {
	sortBy := strings.ToLower(strings.TrimSpace(rawSortBy))
	switch sortBy {
	case "id", "priority", "load", "waiting":
	default:
		sortBy = "priority"
	}

	sortOrder := strings.ToLower(strings.TrimSpace(rawSortOrder))
	if sortOrder != "asc" && sortOrder != "desc" {
		if sortBy == "id" {
			sortOrder = "asc"
		} else {
			sortOrder = "desc"
		}
	}

	sort.SliceStable(records, func(leftIndex int, rightIndex int) bool {
		left := records[leftIndex]
		right := records[rightIndex]
		comparison := 0
		switch sortBy {
		case "id":
			comparison = cmp.Compare(left.channel.Id, right.channel.Id)
		case "load":
			comparison = cmp.Compare(channelLoadMonitorActiveRate(left), channelLoadMonitorActiveRate(right))
		case "waiting":
			comparison = cmp.Compare(left.load.Waiting, right.load.Waiting)
		default:
			comparison = cmp.Compare(channelLoadMonitorPriority(left.channel), channelLoadMonitorPriority(right.channel))
		}
		if comparison == 0 {
			return left.channel.Id < right.channel.Id
		}
		if sortOrder == "asc" {
			return comparison < 0
		}
		return comparison > 0
	})
}

func channelLoadMonitorActiveRate(record channelLoadMonitorRecord) float64 {
	maxConcurrency := record.channel.GetMaxConcurrency()
	if maxConcurrency <= 0 {
		return 0
	}
	return float64(record.load.Active) / float64(maxConcurrency)
}

func channelLoadMonitorPriority(channel *model.Channel) int64 {
	if channel == nil || channel.Priority == nil {
		return 0
	}
	return *channel.Priority
}
