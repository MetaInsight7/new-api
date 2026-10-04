import { describe, expect, test } from 'vitest'

import { channelSchema } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from '../channel-form'
import { parseGroupPriority } from '../group-priority'

describe('group priority overrides', () => {
  test('parses valid partial overrides and ignores malformed entries', () => {
    expect(
      parseGroupPriority(
        '{"vip":{"priority":10},"free":{"weight":2},"bad":null,"invalid":{"weight":-1}}'
      )
    ).toEqual({
      vip: { priority: 10 },
      free: { weight: 2 },
    })
  })

  test('returns an empty override map for invalid JSON values', () => {
    expect(parseGroupPriority('')).toEqual({})
    expect(parseGroupPriority('{"vip":')).toEqual({})
    expect(parseGroupPriority('[]')).toEqual({})
  })

  test('round-trips group priority through channel form defaults and payloads', () => {
    const channel = channelSchema.parse({
      id: 7,
      type: 1,
      key: 'stored-key',
      status: 1,
      name: 'OpenAI',
      created_time: 0,
      test_time: 0,
      response_time: 0,
      balance_updated_time: 0,
      channel_info: {
        is_multi_key: false,
        multi_key_size: 0,
        multi_key_polling_index: 0,
        multi_key_mode: 'random',
      },
      group_priority: '{"vip":{"priority":10,"weight":2}}',
    })
    const defaults = transformChannelToFormDefaults({
      ...channel,
      group: 'default,vip',
      models: 'gpt-5',
    })

    expect(defaults.group_priority).toBe(channel.group_priority)

    const form = channelFormSchema.parse({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'OpenAI',
      key: 'new-key',
      models: 'gpt-5',
      group: ['default', 'vip'],
      group_priority: channel.group_priority,
    })
    expect(transformFormDataToCreatePayload(form).channel.group_priority).toBe(
      channel.group_priority
    )
    expect(
      transformFormDataToUpdatePayload({ ...form, group_priority: '{}' }, 7)
        .group_priority
    ).toBe('{}')
  })
})
