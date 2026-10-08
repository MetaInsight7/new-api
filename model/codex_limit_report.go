package model

import (
	"context"

	"github.com/QuantumNous/new-api/constant"
)

type CodexChannelUsageStat struct {
	ChannelID int   `json:"channel_id" gorm:"column:channel_id"`
	TokenUsed int64 `json:"token_used" gorm:"column:token_used"`
	Quota     int64 `json:"quota" gorm:"column:quota"`
}

func GetCodexLimitReportChannels(ctx context.Context) ([]*Channel, error) {
	channels := make([]*Channel, 0)
	err := DB.WithContext(ctx).Where("type = ?", constant.ChannelTypeCodex).
		Order("priority desc, id asc").Find(&channels).Error
	return channels, err
}

func GetCodexChannelUsageStats(ctx context.Context, channelIDs []int, startTimestamp, endTimestamp int64) (map[int]CodexChannelUsageStat, error) {
	result := make(map[int]CodexChannelUsageStat)
	for offset := 0; offset < len(channelIDs); offset += 500 {
		var stats []CodexChannelUsageStat
		err := LOG_DB.WithContext(ctx).Model(&Log{}).
			Select("channel_id, COALESCE(SUM(prompt_tokens), 0) + COALESCE(SUM(completion_tokens), 0) AS token_used, COALESCE(SUM(quota), 0) AS quota").
			Where("type = ? AND channel_id IN ? AND created_at >= ? AND created_at <= ?",
				LogTypeConsume, channelIDs[offset:min(offset+500, len(channelIDs))], startTimestamp, endTimestamp).
			Group("channel_id").Scan(&stats).Error
		if err != nil {
			return nil, err
		}
		for _, stat := range stats {
			result[stat.ChannelID] = stat
		}
	}
	return result, nil
}
