package common

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigChangedRealRedisTwoClients(t *testing.T) {
	address := os.Getenv("TEST_REDIS_ADDR")
	if address == "" {
		t.Skip("TEST_REDIS_ADDR is required for real Redis verification")
	}
	host, _, err := net.SplitHostPort(address)
	require.NoError(t, err)
	require.True(t, net.ParseIP(host).IsLoopback())
	local := redis.NewClient(&redis.Options{Addr: address, DB: 14})
	peer := redis.NewClient(&redis.Options{Addr: address, DB: 15})
	t.Cleanup(func() { require.NoError(t, local.Close()); require.NoError(t, peer.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	require.NoError(t, local.Ping(ctx).Err())
	require.NoError(t, peer.Ping(ctx).Err())
	server, err := local.Info(ctx, "server").Result()
	require.NoError(t, err)
	for line := range strings.SplitSeq(server, "\n") {
		if version, found := strings.CutPrefix(line, "redis_version:"); found {
			t.Logf("Redis version=%s, two loopback clients, databases=14/15", strings.TrimSpace(version))
		}
	}
	previousClient, previousEnabled := RDB, RedisEnabled
	RDB, RedisEnabled = local, true
	t.Cleanup(func() { RDB, RedisEnabled = previousClient, previousEnabled })
	const topic = "new-api:config-changed:db:14"
	observer := peer.Subscribe(ctx, topic)
	_, err = observer.Receive(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, observer.Close()) })
	notifications := make(chan []int, 16)
	done := make(chan struct{})
	go func() {
		defer close(done)
		SubscribeChannelChanges(ctx, func(channelIDs []int) { notifications <- channelIDs })
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("subscriber did not stop after cancellation")
		}
	})
	require.Eventually(t, func() bool {
		counts, queryErr := peer.PubSubNumSub(ctx, topic).Result()
		return queryErr == nil && counts[topic] == 2
	}, time.Second, 5*time.Millisecond)
	require.NoError(t, PublishConfigChanged(ctx, ConfigScopeChannels, 11, 22, 11))
	message, err := observer.ReceiveMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, topic, message.Channel)
	var payload map[string]RawMessage
	require.NoError(t, Unmarshal([]byte(message.Payload), &payload))
	assert.Len(t, payload, 3)
	assert.JSONEq(t, `"channels"`, string(payload["scope"]))
	assert.JSONEq(t, `"`+GetReplicaID()+`"`, string(payload["source"]))
	assert.JSONEq(t, `[11,22]`, string(payload["close_channel_ids"]))
	require.Error(t, PublishConfigChanged(ctx, "access-token-sentinel"))
	require.Error(t, PublishConfigChanged(ctx, ConfigScopeChannels, -1))
	const peerID = "018f89db-7792-7b5e-a360-7fd9279fd726"
	for _, invalid := range []string{
		`{`, `null`,
		`{"scope":"unknown","source":"` + peerID + `"}`,
		`{"scope":"channels","source":""}`,
		`{"scope":"channels","source":"` + peerID + `","close_channel_ids":[0]}`,
		`{"scope":"channels","source":"` + peerID + `","access_token":"credential-sentinel"}`,
		`{"scope":"options","source":"` + peerID + `"}`,
		`{"scope":"channels","source":"` + peerID + `","close_channel_ids":[` + strings.Repeat("1,", 32*1024) + `1]}`,
	} {
		require.NoError(t, peer.Publish(ctx, topic, invalid).Err())
	}
	validPeer := `{"scope":"channels","source":"` + peerID + `","close_channel_ids":[31,32]}`
	require.NoError(t, peer.Publish(ctx, "new-api:config-changed:db:15", validPeer).Err())
	require.NoError(t, peer.Publish(ctx, topic, validPeer).Err())
	select {
	case channelIDs := <-notifications:
		assert.Equal(t, []int{31, 32}, channelIDs)
	case <-ctx.Done():
		t.Fatal("peer notification was not delivered")
	}
	select {
	case channelIDs := <-notifications:
		t.Errorf("self, invalid, options or other-database event was dispatched: %v", channelIDs)
	default:
	}
}

func TestConfigChangedNoopsWithoutRedis(t *testing.T) {
	previousClient, previousEnabled := RDB, RedisEnabled
	t.Cleanup(func() { RDB, RedisEnabled = previousClient, previousEnabled })
	for _, enabled := range []bool{false, true} {
		RDB, RedisEnabled = nil, enabled
		require.NoError(t, PublishConfigChanged(context.Background(), ConfigScopeChannels, 1))
		SubscribeConfigChanged(context.Background(), func(string) { t.Error("unexpected disabled subscriber callback") })
		SubscribeChannelChanges(context.Background(), func([]int) { t.Error("unexpected disabled channel callback") })
	}
}
