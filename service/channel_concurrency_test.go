package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestChannelConcurrencyMemoryLimit(t *testing.T) {
	previousRedis := common.RDB
	previousRedisEnabled := common.RedisEnabled
	common.RDB = nil
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.RDB = previousRedis
		common.RedisEnabled = previousRedisEnabled
		resetChannelConcurrencyForTest()
	})
	resetChannelConcurrencyForTest()

	channel := &model.Channel{Id: 101, MaxConcurrency: 1}
	first, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NotNil(t, first)

	second, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.False(t, acquired)
	require.Nil(t, second)

	require.NoError(t, ReleaseChannelConcurrency(context.Background(), first))
	third, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NoError(t, ReleaseChannelConcurrency(context.Background(), third))
}

func TestChannelConcurrencyUnlimitedDoesNotAllocateLease(t *testing.T) {
	previousRedis := common.RDB
	previousRedisEnabled := common.RedisEnabled
	common.RDB = nil
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.RDB = previousRedis
		common.RedisEnabled = previousRedisEnabled
		resetChannelConcurrencyForTest()
	})
	resetChannelConcurrencyForTest()

	for range 3 {
		lease, acquired, err := TryAcquireChannelConcurrency(context.Background(), &model.Channel{Id: 102})
		require.NoError(t, err)
		require.True(t, acquired)
		require.Nil(t, lease)
	}
}
