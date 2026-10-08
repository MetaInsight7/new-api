package service

import (
	"context"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/wsmanager"
)

var channelCacheNotificationsStarted sync.Once

func StartChannelCacheNotifications(ctx context.Context) {
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return
	}
	channelCacheNotificationsStarted.Do(func() {
		go common.SubscribeChannelChanges(ctx, HandleChannelCacheNotification)
	})
}

func HandleChannelCacheNotification(closeChannelIDs []int) {
	defer func() {
		if recover() != nil {
			common.SysError("channel-cache notification handler panicked; periodic cache sync remains available")
		}
	}()
	model.InitChannelCache()
	wsmanager.CloseChannels(closeChannelIDs, ChannelDisabledCloseReason)
}
