package model

import (
	"fmt"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// GetSatisfiedChannelCandidates returns every enabled channel in the selected
// priority for one group/model request. Callers use this when runtime routing
// needs to inspect all candidates, while GetRandomSatisfiedChannel preserves
// the existing weighted choice for ordinary requests.
func GetSatisfiedChannelCandidates(group string, modelName string, retry int, filters []dto.ChannelFilter) ([]*Channel, error) {
	if common.MemoryCacheEnabled {
		return getCachedSatisfiedChannelCandidates(group, modelName, retry, filters), nil
	}

	var abilities []Ability
	if err := DB.Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, modelName, true).
		Order("priority DESC, weight DESC").Find(&abilities).Error; err != nil {
		return nil, err
	}
	abilities = filterAbilitiesByConstraints(abilities, modelName, filters)
	if len(abilities) == 0 {
		return nil, nil
	}

	ids := make([]int, 0, len(abilities))
	seenIDs := make(map[int]struct{}, len(abilities))
	for _, ability := range abilities {
		if _, seen := seenIDs[ability.ChannelId]; !seen {
			seenIDs[ability.ChannelId] = struct{}{}
			ids = append(ids, ability.ChannelId)
		}
	}
	var channels []*Channel
	if err := DB.Where("id IN ?", ids).Find(&channels).Error; err != nil {
		return nil, err
	}
	byID := make(map[int]*Channel, len(channels))
	for _, channel := range channels {
		byID[channel.Id] = channel
	}

	priorities := make([]int64, 0)
	seenPriorities := make(map[int64]struct{})
	for _, ability := range abilities {
		channel, ok := byID[ability.ChannelId]
		if !ok {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", ability.ChannelId)
		}
		priority := channel.GetPriorityForGroup(group)
		if _, seen := seenPriorities[priority]; !seen {
			seenPriorities[priority] = struct{}{}
			priorities = append(priorities, priority)
		}
	}
	sort.Slice(priorities, func(i, j int) bool { return priorities[i] > priorities[j] })
	if retry < 0 {
		retry = 0
	}
	if retry >= len(priorities) {
		return nil, nil
	}
	targetPriority := priorities[retry]

	ids = ids[:0]
	seenIDs = make(map[int]struct{})
	for _, ability := range abilities {
		channel, ok := byID[ability.ChannelId]
		if !ok {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", ability.ChannelId)
		}
		priority := channel.GetPriorityForGroup(group)
		if priority != targetPriority {
			continue
		}
		if _, seen := seenIDs[ability.ChannelId]; !seen {
			seenIDs[ability.ChannelId] = struct{}{}
			ids = append(ids, ability.ChannelId)
		}
	}
	ordered := make([]*Channel, 0, len(ids))
	for _, id := range ids {
		if channel := byID[id]; channel != nil {
			ordered = append(ordered, channel)
		}
	}
	return ordered, nil
}

func getCachedSatisfiedChannelCandidates(group string, modelName string, retry int, filters []dto.ChannelFilter) []*Channel {
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	ids, _ := filterCandidateIDs(group2model2channels[group][modelName], modelName, filters)
	if len(ids) == 0 {
		normalizedModel := ratio_setting.RoutingMatchModelName(modelName)
		ids, _ = filterCandidateIDs(group2model2channels[group][normalizedModel], modelName, filters)
	}
	if len(ids) == 0 {
		return nil
	}

	priorities := make([]int64, 0)
	seenPriorities := make(map[int64]struct{})
	for _, id := range ids {
		channel := channelsIDM[id]
		if channel == nil {
			continue
		}
		priority := channel.GetPriorityForGroup(group)
		if _, seen := seenPriorities[priority]; !seen {
			seenPriorities[priority] = struct{}{}
			priorities = append(priorities, priority)
		}
	}
	sort.Slice(priorities, func(i, j int) bool { return priorities[i] > priorities[j] })
	if len(priorities) == 0 {
		return nil
	}
	if retry < 0 {
		retry = 0
	}
	if retry >= len(priorities) {
		return nil
	}
	targetPriority := priorities[retry]

	channels := make([]*Channel, 0, len(ids))
	for _, id := range ids {
		channel := channelsIDM[id]
		if channel != nil && channel.GetPriorityForGroup(group) == targetPriority {
			channels = append(channels, channel)
		}
	}
	return channels
}
