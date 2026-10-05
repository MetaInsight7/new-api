/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import { describe, expect, test } from 'vitest'

import { CHANNEL_TYPE_CODEX } from '../../constants'
import type { Channel } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  buildSettingJSON,
  transformChannelToFormDefaults,
} from '../channel-form'

const codexChannel = (setting: string): Channel =>
  ({
    ...CHANNEL_FORM_DEFAULT_VALUES,
    id: 57,
    type: CHANNEL_TYPE_CODEX,
    key: '',
    name: 'Codex',
    group: 'default',
    created_time: 0,
    test_time: 0,
    response_time: 0,
    balance: 0,
    balance_updated_time: 0,
    setting,
    channel_info: {
      is_multi_key: false,
      multi_key_size: 0,
      multi_key_polling_index: 0,
      multi_key_mode: 'random',
    },
  }) as Channel

describe('Codex image carrier settings', () => {
  test('round-trips the per-channel carrier model through edit defaults', () => {
    const defaults = transformChannelToFormDefaults(
      codexChannel('{"image_carrier_model":"gpt-5.5"}')
    )

    expect(defaults.image_carrier_model).toBe('gpt-5.5')
    expect(JSON.parse(buildSettingJSON(defaults))).toMatchObject({
      image_carrier_model: 'gpt-5.5',
    })
  })

  test('omits a cleared carrier model from the saved settings', () => {
    const defaults = transformChannelToFormDefaults(
      codexChannel('{"image_carrier_model":"gpt-5.5"}')
    )
    defaults.image_carrier_model = '  '

    expect(JSON.parse(buildSettingJSON(defaults))).not.toHaveProperty(
      'image_carrier_model'
    )
  })

  test('does not write the carrier setting for other channel types', () => {
    const defaults = {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      type: 1,
      image_carrier_model: 'gpt-5.5',
    }

    expect(JSON.parse(buildSettingJSON(defaults))).not.toHaveProperty(
      'image_carrier_model'
    )
  })
})

describe('Codex fingerprint settings', () => {
  test('defaults fingerprint mode to off', () => {
    const defaults = transformChannelToFormDefaults(codexChannel('{}'))

    expect(defaults.codex_fingerprint_mode).toBe('off')
    expect(JSON.parse(buildSettingJSON(defaults))).not.toHaveProperty(
      'codex_fingerprint_mode'
    )
  })

  test('round-trips an explicitly selected fingerprint mode', () => {
    const defaults = transformChannelToFormDefaults(
      codexChannel(
        '{"proxy":"http://proxy.example","codex_fingerprint_mode":"full"}'
      )
    )

    expect(defaults.codex_fingerprint_mode).toBe('full')
    expect(JSON.parse(buildSettingJSON(defaults))).toMatchObject({
      proxy: 'http://proxy.example',
      codex_fingerprint_mode: 'full',
    })
  })

  test('clearing fingerprint mode preserves unrelated settings', () => {
    const defaults = transformChannelToFormDefaults(
      codexChannel(
        '{"proxy":"http://proxy.example","codex_fingerprint_mode":"session"}'
      )
    )
    defaults.codex_fingerprint_mode = 'off'

    expect(JSON.parse(buildSettingJSON(defaults))).toMatchObject({
      proxy: 'http://proxy.example',
    })
    expect(JSON.parse(buildSettingJSON(defaults))).not.toHaveProperty(
      'codex_fingerprint_mode'
    )
  })

  test('does not write fingerprint mode for another channel type', () => {
    const defaults = {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      type: 1,
      codex_fingerprint_mode: 'full' as const,
    }

    expect(JSON.parse(buildSettingJSON(defaults))).not.toHaveProperty(
      'codex_fingerprint_mode'
    )
  })
})
