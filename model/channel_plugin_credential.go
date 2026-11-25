package model

import (
	"context"
	"errors"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/google/uuid"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Fixed reasons the host records when it disables a task plugin channel, or
// one of its keys, for a credential only a login can restore. A successful
// login re-enables what was disabled for one of them; a manual disable stays.
const (
	ChannelStatusReasonPluginLoginRequired = "Plugin credential login required"
	ChannelStatusReasonPluginNeedsLogin    = "Plugin credential needs a new login"
)

// ErrChannelKeyChanged reports a channel key that changed while a locked
// update waited, which only a write outside UpdateChannelLocked can do.
var ErrChannelKeyChanged = errors.New("channel key changed during the credential update")
var ErrChannelConfigChanged = errors.New("channel configuration changed during key validation")

// SQLite serves one node, so process locks stand in for its row locks.
var (
	sqliteChannelKeyLocks sync.Map // channel id -> *sync.Mutex
	sqlitePluginLoginLock sync.Mutex
)

func sqliteChannelKeyLock(channelID int) *sync.Mutex {
	lock, _ := sqliteChannelKeyLocks.LoadOrStore(channelID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func channelConfigUpdates(channel *Channel) map[string]any {
	return map[string]any{
		"type":                   channel.Type,
		"key":                    channel.Key,
		"open_ai_organization":   channel.OpenAIOrganization,
		"test_model":             channel.TestModel,
		"name":                   channel.Name,
		"weight":                 channel.Weight,
		"max_concurrency":        channel.MaxConcurrency,
		"base_url":               channel.BaseURL,
		"other":                  channel.Other,
		"models":                 channel.Models,
		"group":                  channel.Group,
		"model_mapping":          channel.ModelMapping,
		"status_code_mapping":    channel.StatusCodeMapping,
		"priority":               channel.Priority,
		"auto_ban":               channel.AutoBan,
		"tag":                    channel.Tag,
		"setting":                channel.Setting,
		"param_override":         channel.ParamOverride,
		"header_override":        channel.HeaderOverride,
		"group_priority":         channel.GroupPriority,
		"remark":                 channel.Remark,
		"channel_info":           channel.ChannelInfo,
		"settings":               channel.OtherSettings,
		"codex_fingerprint_seed": channel.CodexFingerprintSeed,
	}
}

func UpdateChannelConfigLocked(channelID int, expectedKey *string, update func(channel *Channel) error) (before *Channel, after *Channel, changed bool, err error) {
	pollingLock := GetChannelPollingLock(channelID)
	pollingLock.Lock()
	defer pollingLock.Unlock()
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		keyLock := sqliteChannelKeyLock(channelID)
		keyLock.Lock()
		defer keyLock.Unlock()
	}

	var current Channel
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", channelID).First(&current).Error; err != nil {
			return err
		}
		stored := current
		before = &stored
		if expectedKey != nil && current.Key != *expectedKey {
			return ErrChannelKeyChanged
		}

		serverStatus := current.Status
		serverCreatedTime := current.CreatedTime
		serverTestTime := current.TestTime
		serverResponseTime := current.ResponseTime
		serverBalance := current.Balance
		serverBalanceUpdatedTime := current.BalanceUpdatedTime
		serverUsedQuota := current.UsedQuota
		serverOtherInfo := current.OtherInfo
		serverFingerprintSeed := current.CodexFingerprintSeed
		channelInfoJSON, err := common.Marshal(current.ChannelInfo)
		if err != nil {
			return err
		}
		beforeConfig, err := common.Marshal(channelConfigUpdates(&current))
		if err != nil {
			return err
		}
		if err := update(&current); err != nil {
			return err
		}

		requestedMultiKeyMode := current.ChannelInfo.MultiKeyMode
		current.Status = serverStatus
		current.CreatedTime = serverCreatedTime
		current.TestTime = serverTestTime
		current.ResponseTime = serverResponseTime
		current.Balance = serverBalance
		current.BalanceUpdatedTime = serverBalanceUpdatedTime
		current.UsedQuota = serverUsedQuota
		current.OtherInfo = serverOtherInfo
		if err := common.Unmarshal(channelInfoJSON, &current.ChannelInfo); err != nil {
			return err
		}
		current.ChannelInfo.MultiKeyMode = requestedMultiKeyMode
		current.Keys = nil
		if current.ChannelInfo.IsMultiKey {
			keys := current.GetKeys()
			current.ChannelInfo.MultiKeySize = len(keys)
			if len(keys) == 0 || current.ChannelInfo.MultiKeyPollingIndex >= len(keys) {
				current.ChannelInfo.MultiKeyPollingIndex = 0
			}
			for index := range current.ChannelInfo.MultiKeyStatusList {
				if index >= len(keys) {
					delete(current.ChannelInfo.MultiKeyStatusList, index)
					delete(current.ChannelInfo.MultiKeyDisabledReason, index)
					delete(current.ChannelInfo.MultiKeyDisabledTime, index)
				}
			}
		}

		switch {
		case current.Type != constant.ChannelTypeCodex:
			current.CodexFingerprintSeed = ""
		case validCodexFingerprintSeed(serverFingerprintSeed):
			current.CodexFingerprintSeed = serverFingerprintSeed
		case current.Status != common.ChannelStatusEnabled:
			current.CodexFingerprintSeed = ""
		default:
			current.CodexFingerprintSeed = uuid.NewString()
		}

		afterConfig, err := common.Marshal(channelConfigUpdates(&current))
		if err != nil {
			return err
		}
		if string(beforeConfig) == string(afterConfig) {
			after = &current
			return nil
		}
		if err := tx.Model(&Channel{}).Where("id = ?", channelID).Updates(channelConfigUpdates(&current)).Error; err != nil {
			return err
		}
		if err := current.UpdateAbilities(tx); err != nil {
			return err
		}
		changed = true
		after = &current
		return nil
	})
	if err != nil || !changed {
		return before, after, false, err
	}
	syncCommittedChannelChange()
	return before, after, true, nil
}

// UpdateChannelLocked lets update change a channel's key while no other node
// or goroutine can: MySQL and PostgreSQL hold the channel row lock across
// update, and SQLite holds a process lock. update must not wait on the
// network, because every request of a multi-key channel waits on this lock.
// Every write of a channel's key goes through here or through
// UpdateTaskPluginChannelLocked, which takes the same locks; Channel.Update
// never writes it. So the key update reads is the stored one, never a copy a
// refresh has replaced since.
// update receives the stored channel and may change its Key, Status,
// OtherInfo and ChannelInfo; the changed ones are written together. The
// stored channel afterwards is returned, a new key is published to this
// node's channel cache, and a new status to the channel's abilities.
func UpdateChannelLocked(channelID int, update func(channel *Channel) error) (*Channel, error) {
	pollingLock := GetChannelPollingLock(channelID)
	pollingLock.Lock()
	defer pollingLock.Unlock()
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		lock := sqliteChannelKeyLock(channelID)
		lock.Lock()
		defer lock.Unlock()
	}

	var channel Channel
	var statusChanged, written bool
	err := DB.Session(&gorm.Session{Logger: logger.Discard}).Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", channelID).First(&channel).Error; err != nil {
			return err
		}
		key, status, otherInfo := channel.Key, channel.Status, channel.OtherInfo
		info, err := common.Marshal(channel.ChannelInfo)
		if err != nil {
			return err
		}
		if err := update(&channel); err != nil {
			return err
		}
		channel.Keys = nil
		updates := map[string]any{}
		if channel.Key != key {
			updates["key"] = channel.Key
		}
		if channel.Status != status {
			updates["status"], statusChanged = channel.Status, true
		}
		if channel.OtherInfo != otherInfo {
			updates["other_info"] = channel.OtherInfo
		}
		if next, err := common.Marshal(channel.ChannelInfo); err != nil {
			return err
		} else if string(next) != string(info) {
			updates["channel_info"] = channel.ChannelInfo
		}
		if len(updates) == 0 {
			return nil
		}
		if err := tx.Model(&Channel{}).Where("id = ?", channelID).Updates(updates).Error; err != nil {
			return err
		}
		if statusChanged {
			if err := updateAbilityStatusWithDB(tx, channelID, channel.Status == common.ChannelStatusEnabled); err != nil {
				return err
			}
			if channel.Type == constant.ChannelTypeCodex && channel.Status == common.ChannelStatusEnabled {
				if _, err := ensureCodexFingerprintSeedTx(tx, channelID); err != nil {
					return err
				}
			}
		}
		written = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if written {
		var closeChannelIDs []int
		if statusChanged && channel.Status != common.ChannelStatusEnabled {
			closeChannelIDs = []int{channelID}
		}
		syncCommittedChannelChange(closeChannelIDs...)
	}
	return &channel, nil
}

// UpdateTaskPluginChannelLocked runs update on a task plugin channel while
// every other task plugin channel is locked too, so a check across channels,
// such as one account per plugin, cannot interleave with another login's
// write. update may change Key, Status, OtherInfo and ChannelInfo and must not
// wait on the network. The caller reloads the channel cache.
func UpdateTaskPluginChannelLocked(channelID int, update func(channel *Channel, others []*Channel) error) error {
	// Key status and the polling cursor share channel_info; see UpdateChannelStatus.
	pollingLock := GetChannelPollingLock(channelID)
	pollingLock.Lock()
	defer pollingLock.Unlock()
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		sqlitePluginLoginLock.Lock()
		defer sqlitePluginLoginLock.Unlock()
		keyLock := sqliteChannelKeyLock(channelID)
		keyLock.Lock()
		defer keyLock.Unlock()
	}
	var statusChanged, written bool
	var finalStatus int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var channels []*Channel
		if err := lockForUpdate(tx).Where("type = ?", constant.ChannelTypeTaskPlugin).Order("id").Find(&channels).Error; err != nil {
			return err
		}
		var target *Channel
		others := make([]*Channel, 0, len(channels))
		for _, channel := range channels {
			if channel.Id == channelID {
				target = channel
				continue
			}
			others = append(others, channel)
		}
		if target == nil {
			return gorm.ErrRecordNotFound
		}
		key, status, otherInfo := target.Key, target.Status, target.OtherInfo
		channelInfo, err := common.Marshal(target.ChannelInfo)
		if err != nil {
			return err
		}
		if err := update(target, others); err != nil {
			return err
		}
		target.Keys = nil
		updates := map[string]any{}
		if target.Key != key {
			updates["key"] = target.Key
		}
		if target.Status != status {
			updates["status"], statusChanged = target.Status, true
		}
		if target.OtherInfo != otherInfo {
			updates["other_info"] = target.OtherInfo
		}
		if next, err := common.Marshal(target.ChannelInfo); err != nil {
			return err
		} else if string(next) != string(channelInfo) {
			updates["channel_info"] = target.ChannelInfo
		}
		if len(updates) == 0 {
			return nil
		}
		if err := tx.Model(&Channel{}).Where("id = ?", channelID).Updates(updates).Error; err != nil {
			return err
		}
		if statusChanged {
			if err := updateAbilityStatusWithDB(tx, channelID, target.Status == common.ChannelStatusEnabled); err != nil {
				return err
			}
		}
		written = true
		finalStatus = target.Status
		return nil
	})
	if err != nil || !written {
		return err
	}
	var closeChannelIDs []int
	if statusChanged && finalStatus != common.ChannelStatusEnabled {
		closeChannelIDs = []int{channelID}
	}
	syncCommittedChannelChange(closeChannelIDs...)
	return nil
}

// CacheUpdateChannelKey publishes a new key for a cached channel. The cached
// channel is replaced by a copy, so a request holding the previous one keeps
// reading a consistent key.
func CacheUpdateChannelKey(id int, key string) {
	if !common.MemoryCacheEnabled {
		return
	}
	channelSyncLock.Lock()
	defer channelSyncLock.Unlock()
	cached, ok := channelsIDM[id]
	if !ok {
		return
	}
	updated := *cached
	updated.Key = key
	updated.Keys = nil
	updated.Keys = updated.GetKeys()
	channelsIDM[id] = &updated
}

// channelRelayConfig returns the relay routes of a task plugin channel's
// plugin, nil for any other channel. It reads the setting without GetSetting,
// which clears the stored setting when it is invalid, so a pricing read never
// writes the channel.
func channelRelayConfig(ch *Channel) *dto.AdvancedCustomConfig {
	var setting dto.ChannelSettings
	if ch.Type != constant.ChannelTypeTaskPlugin || ch.Setting == nil || common.UnmarshalJsonStr(*ch.Setting, &setting) != nil {
		return nil
	}
	plugin, ok := jsplugin.DefaultRegistry.Get(setting.TaskPluginKey)
	if !ok {
		return nil
	}
	return plugin.Meta.RelayConfig()
}

// UpdateScheduledChannelLocked persists a completed scheduled result. update
// only compares the prepared key and applies validated data to the current row;
// it must not run JavaScript or perform network I/O. The polling/key locks and
// database transaction therefore cover only the short persistence step.
func UpdateScheduledChannelLocked(ctx context.Context, channelID int, update func(channel *Channel) error) (*Channel, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pollingLock := GetChannelPollingLock(channelID)
	pollingLock.Lock()
	defer pollingLock.Unlock()
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		keyLock := sqliteChannelKeyLock(channelID)
		keyLock.Lock()
		defer keyLock.Unlock()
	}
	var channel Channel
	var statusChanged, written bool
	// Failed SQL must not include a credential, including in debug mode.
	db := DB.WithContext(ctx).Session(&gorm.Session{Logger: logger.Discard})
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", channelID).First(&channel).Error; err != nil {
			return err
		}
		key, status, otherInfo := channel.Key, channel.Status, channel.OtherInfo
		info, err := common.Marshal(channel.ChannelInfo)
		if err != nil {
			return err
		}
		if err := update(&channel); err != nil {
			return err
		}
		channel.Keys = nil
		updates := map[string]any{}
		if channel.Key != key {
			updates["key"] = channel.Key
		}
		if channel.Status != status {
			updates["status"], statusChanged = channel.Status, true
		}
		if channel.OtherInfo != otherInfo {
			updates["other_info"] = channel.OtherInfo
		}
		if next, err := common.Marshal(channel.ChannelInfo); err != nil {
			return err
		} else if string(next) != string(info) {
			updates["channel_info"] = channel.ChannelInfo
		}
		if len(updates) == 0 {
			return nil
		}
		if err := tx.Model(&Channel{}).Where("id = ?", channelID).Updates(updates).Error; err != nil {
			return err
		}
		if statusChanged {
			if err := updateAbilityStatusWithDB(tx, channelID, channel.Status == common.ChannelStatusEnabled); err != nil {
				return err
			}
		}
		written = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !written {
		return &channel, nil
	}
	var closeChannelIDs []int
	if statusChanged && channel.Status != common.ChannelStatusEnabled {
		closeChannelIDs = []int{channelID}
	}
	syncCommittedChannelChange(closeChannelIDs...)
	return &channel, nil
}

// SetScheduledChannelKeyStatus applies an automatic status transition to the
// stored key. Call it only inside UpdateScheduledChannelLocked, after locating
// the key again; a manual disable can never be undone by a plugin.
func SetScheduledChannelKeyStatus(channel *Channel, keyIndex, status int, reason string) bool {
	if channel.Status != common.ChannelStatusEnabled && channel.Status != common.ChannelStatusAutoDisabled {
		return false
	}
	channel.Keys = nil
	keys := []string{channel.Key}
	if channel.ChannelInfo.IsMultiKey {
		keys = channel.GetKeys()
	}
	if keyIndex < 0 || keyIndex >= len(keys) {
		return false
	}
	current := channel.Status
	if channel.ChannelInfo.IsMultiKey {
		current = common.ChannelStatusEnabled
		if stored, ok := channel.ChannelInfo.MultiKeyStatusList[keyIndex]; ok {
			current = stored
		}
	} else if keyIndex != 0 {
		return false
	}
	switch status {
	case common.ChannelStatusEnabled:
		if current != common.ChannelStatusAutoDisabled {
			return false
		}
	case common.ChannelStatusAutoDisabled:
		if current != common.ChannelStatusEnabled || !channel.GetAutoBan() {
			return false
		}
	default:
		return false
	}
	if channel.ChannelInfo.IsMultiKey {
		handlerMultiKeyUpdate(channel, keys[keyIndex], status, reason)
		if status == common.ChannelStatusEnabled {
			delete(channel.ChannelInfo.MultiKeyDisabledReason, keyIndex)
			delete(channel.ChannelInfo.MultiKeyDisabledTime, keyIndex)
		}
	} else {
		channel.Status = status
		info := channel.GetOtherInfo()
		info["status_reason"] = reason
		info["status_time"] = common.GetTimestamp()
		channel.SetOtherInfo(info)
	}
	return true
}
