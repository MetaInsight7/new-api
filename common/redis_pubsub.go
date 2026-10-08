package common

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

const (
	ConfigChangedChannel = "new-api:config-changed"
	ConfigScopeOptions   = "options"
	ConfigScopeChannels  = "channels"
	maxConfigChangeBytes = 64 * 1024
)

var configChangeReplicaID = sync.OnceValue(uuid.NewString)

type configChangeMessage struct {
	Scope           string `json:"scope"`
	Source          string `json:"source"`
	CloseChannelIDs []int  `json:"close_channel_ids,omitempty"`
}

func GetReplicaID() string {
	return configChangeReplicaID()
}

func PublishConfigChanged(ctx context.Context, scope string, closeChannelIDs ...int) error {
	client := RDB
	if !RedisEnabled || client == nil {
		return nil
	}
	channelIDs := slices.Clone(closeChannelIDs)
	slices.Sort(channelIDs)
	message := configChangeMessage{Scope: scope, Source: GetReplicaID(), CloseChannelIDs: slices.Compact(channelIDs)}
	if err := validateConfigChangeMessage(message); err != nil {
		return err
	}
	payload, err := Marshal(message)
	if err != nil {
		return err
	}
	if len(payload) > maxConfigChangeBytes {
		return errors.New("config-change notification exceeds size limit")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return client.Publish(ctx, configChangedTopic(client.Options().DB), payload).Err()
}

func SubscribeConfigChanged(ctx context.Context, handler func(scope string)) {
	if handler == nil {
		return
	}
	subscribeConfigChanged(ctx, func(message configChangeMessage) { handler(message.Scope) })
}

func SubscribeChannelChanges(ctx context.Context, handler func(closeChannelIDs []int)) {
	if handler == nil {
		return
	}
	subscribeConfigChanged(ctx, func(message configChangeMessage) {
		if message.Scope == ConfigScopeChannels {
			handler(message.CloseChannelIDs)
		}
	})
}

func configChangedTopic(database int) string {
	return ConfigChangedChannel + ":db:" + strconv.Itoa(database)
}

func validateConfigChangeMessage(message configChangeMessage) error {
	if message.Scope != ConfigScopeOptions && message.Scope != ConfigScopeChannels {
		return errors.New("unsupported config-change scope")
	}
	source, err := uuid.Parse(message.Source)
	if err != nil || source == uuid.Nil || source.String() != message.Source {
		return errors.New("invalid config-change source")
	}
	if message.Scope != ConfigScopeChannels && len(message.CloseChannelIDs) != 0 {
		return errors.New("websocket close IDs require channels scope")
	}
	for _, channelID := range message.CloseChannelIDs {
		if channelID <= 0 {
			return errors.New("invalid config-change channel ID")
		}
	}
	return nil
}

func subscribeConfigChanged(ctx context.Context, handler func(configChangeMessage)) {
	client := RDB
	if !RedisEnabled || client == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	subscription := client.Subscribe(ctx, configChangedTopic(client.Options().DB))
	defer subscription.Close()
	messages := subscription.Channel(redis.WithChannelSize(16))
	self := GetReplicaID()
	for {
		select {
		case <-ctx.Done():
			return
		case message, open := <-messages:
			if !open || ctx.Err() != nil {
				return
			}
			if len(message.Payload) > maxConfigChangeBytes {
				continue
			}
			var fields map[string]RawMessage
			if err := Unmarshal([]byte(message.Payload), &fields); err != nil || fields == nil {
				continue
			}
			unknownField := false
			for field := range fields {
				if field != "scope" && field != "source" && field != "close_channel_ids" {
					unknownField = true
					break
				}
			}
			var notification configChangeMessage
			if unknownField || Unmarshal([]byte(message.Payload), &notification) != nil || validateConfigChangeMessage(notification) != nil {
				continue
			}
			if notification.Source == self {
				continue
			}
			handler(notification)
		}
	}
}
