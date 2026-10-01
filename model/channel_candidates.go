package model

import (
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

	priorities := make([]int64, 0)
	seenPriorities := make(map[int64]struct{})
	for _, ability := range abilities {
		priority := int64(0)
		if ability.Priority != nil {
			priority = *ability.Priority
		}
		if _, seen := seenPriorities[priority]; !seen {
			seenPriorities[priority] = struct{}{}
			priorities = append(priorities, priority)
		}
	}
	if retry >= len(priorities) {
		retry = len(priorities) - 1
	}
	targetPriority := priorities[retry]

	ids := make([]int, 0, len(abilities))
	seenIDs := make(map[int]struct{})
	for _, ability := range abilities {
		priority := int64(0)
		if ability.Priority != nil {
			priority = *ability.Priority
		}
		if priority != targetPriority {
			continue
		}
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
		priority := channel.GetPriority()
		if _, seen := seenPriorities[priority]; !seen {
			seenPriorities[priority] = struct{}{}
			priorities = append(priorities, priority)
		}
	}
	sort.Slice(priorities, func(i, j int) bool { return priorities[i] > priorities[j] })
	if len(priorities) == 0 {
		return nil
	}
	if retry >= len(priorities) {
		retry = len(priorities) - 1
	}
	targetPriority := priorities[retry]

	channels := make([]*Channel, 0, len(ids))
	for _, id := range ids {
		channel := channelsIDM[id]
		if channel != nil && channel.GetPriority() == targetPriority {
			channels = append(channels, channel)
		}
	}
	return channels
}
