import type { ChannelConcurrencySettingsValues } from '../types'

export const defaultChannelConcurrencySettings: ChannelConcurrencySettingsValues =
  {
    'channel_concurrency_setting.wait_enabled': true,
    'channel_concurrency_setting.wait_timeout_ms': 5000,
    'channel_concurrency_setting.max_waiting_per_channel': 0,
    'channel_concurrency_setting.cooldown_enabled': true,
    'channel_concurrency_setting.cooldown_seconds': 30,
    'channel_concurrency_setting.cooldown_on_status_429': true,
    'channel_concurrency_setting.cooldown_on_message_match': false,
    'channel_concurrency_setting.load_cache_enabled': true,
    'channel_concurrency_setting.slot_ttl_minutes': 30,
    'channel_concurrency_setting.wait_interval_ms': 100,
    'channel_concurrency_setting.load_cache_ttl_ms': 200,
    'channel_concurrency_setting.max_acquire_attempts': 8,
  }
