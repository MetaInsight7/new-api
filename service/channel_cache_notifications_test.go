package service

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/wsmanager"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestChannelCacheNotificationsRealRedisReloadAndLocalWebSocketClose(t *testing.T) {
	address := os.Getenv("TEST_REDIS_ADDR")
	if address == "" {
		t.Skip("TEST_REDIS_ADDR is required for real Redis verification")
	}
	host, _, err := net.SplitHostPort(address)
	require.NoError(t, err)
	require.True(t, net.ParseIP(host).IsLoopback())
	local := redis.NewClient(&redis.Options{Addr: address, DB: 14})
	peer := redis.NewClient(&redis.Options{Addr: address, DB: 14})
	t.Cleanup(func() { require.NoError(t, local.Close()); require.NoError(t, peer.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	require.NoError(t, local.Ping(ctx).Err())
	require.NoError(t, peer.Ping(ctx).Err())
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "channel-cache.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Ability{}))
	previousDB, previousMemory := model.DB, common.MemoryCacheEnabled
	previousRedis, previousClient := common.RedisEnabled, common.RDB
	model.DB, common.MemoryCacheEnabled = database, true
	common.RedisEnabled, common.RDB = false, local
	t.Cleanup(func() {
		model.DB, common.MemoryCacheEnabled = previousDB, previousMemory
		common.RedisEnabled, common.RDB = previousRedis, previousClient
		if previousDB != nil {
			model.InitChannelCache()
		}
		connection, connectionErr := database.DB()
		require.NoError(t, connectionErr)
		require.NoError(t, connection.Close())
	})
	disabled := &model.Channel{Name: "remote-disabled", Type: 1, Key: "credential-sentinel", Status: 1, Models: "cache-model", Group: "default"}
	unchanged := &model.Channel{Name: "remote-unchanged", Type: 1, Key: "other-credential-sentinel", Status: 1, Models: "cache-model", Group: "default"}
	require.NoError(t, database.Create(disabled).Error)
	require.NoError(t, database.Create(unchanged).Error)
	require.NoError(t, database.Create(&model.Ability{ChannelId: disabled.Id, Model: "cache-model", Group: "default", Enabled: true}).Error)
	model.InitChannelCache()
	const topic = "new-api:config-changed:db:14"
	observer := peer.Subscribe(ctx, topic, "new-api:wsmanager:channel-close:db:14")
	for range 2 {
		_, err = observer.Receive(ctx)
		require.NoError(t, err)
	}
	t.Cleanup(func() { require.NoError(t, observer.Close()) })
	StartChannelCacheNotifications(ctx)
	counts, err := peer.PubSubNumSub(ctx, topic).Result()
	require.NoError(t, err)
	assert.EqualValues(t, 1, counts[topic], "disabled startup must not register a subscriber")
	common.RedisEnabled = true
	StartChannelCacheNotifications(ctx)
	StartChannelCacheNotifications(ctx)
	t.Cleanup(func() {
		cancel()
		require.Eventually(t, func() bool {
			counts, queryErr := peer.PubSubNumSub(context.Background(), topic).Result()
			return queryErr == nil && counts[topic] == 1
		}, time.Second, 5*time.Millisecond)
	})
	require.Eventually(t, func() bool {
		counts, queryErr := peer.PubSubNumSub(ctx, topic).Result()
		return queryErr == nil && counts[topic] == 2
	}, time.Second, 5*time.Millisecond)
	closed := make(chan string, 2)
	t.Cleanup(wsmanager.Register(disabled.Id, wsmanager.KindResponses, func(reason string) { closed <- reason }))
	t.Cleanup(wsmanager.Register(unchanged.Id, wsmanager.KindRealtime, func(string) { closed <- "unrelated websocket closed" }))
	require.NoError(t, database.Transaction(func(transaction *gorm.DB) error {
		if err := transaction.Model(&model.Channel{}).Where("id = ?", disabled.Id).Update("status", 4).Error; err != nil {
			return err
		}
		return transaction.Model(&model.Ability{}).Where("channel_id = ?", disabled.Id).Update("enabled", false).Error
	}))
	require.NoError(t, common.PublishConfigChanged(ctx, common.ConfigScopeChannels, disabled.Id))
	selfMessage, err := observer.ReceiveMessage(ctx)
	require.NoError(t, err)
	assert.NotContains(t, selfMessage.Payload, "credential-sentinel")
	const peerID = "018f89db-7792-7b5e-a360-7fd9279fd726"
	payload := fmt.Sprintf(`{"scope":"channels","source":"%s","close_channel_ids":[%d]}`, peerID, disabled.Id)
	require.NoError(t, peer.Publish(ctx, topic, payload).Err())
	message, err := observer.ReceiveMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, topic, message.Channel)
	assert.JSONEq(t, payload, message.Payload)
	select {
	case reason := <-closed:
		assert.Equal(t, ChannelDisabledCloseReason, reason)
	case <-ctx.Done():
		t.Fatal("peer event did not close the local websocket")
	}
	cached, err := model.CacheGetChannel(disabled.Id)
	require.NoError(t, err)
	assert.Equal(t, 4, cached.Status)
	require.NoError(t, database.Model(&model.Channel{}).Where("id = ?", unchanged.Id).Update("models", "refreshed-model").Error)
	ordinaryChange := `{"scope":"channels","source":"` + peerID + `"}`
	require.NoError(t, peer.Publish(ctx, topic, ordinaryChange).Err())
	message, err = observer.ReceiveMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, topic, message.Channel)
	assert.JSONEq(t, ordinaryChange, message.Payload)
	require.Eventually(t, func() bool {
		cached, cacheErr := model.CacheGetChannel(unchanged.Id)
		return cacheErr == nil && cached.Models == "refreshed-model"
	}, time.Second, 5*time.Millisecond)
	require.NoError(t, peer.Publish(ctx, topic, "notification-barrier").Err())
	message, err = observer.ReceiveMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "notification-barrier", message.Payload, "consumer must not publish cache or websocket events")
	select {
	case reason := <-closed:
		t.Errorf("ordinary reload closed an unrelated websocket: %s", reason)
	default:
	}
}
