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

import type { UserModelAccess } from '@/features/available-models'

import {
  getApiKeyModelAccessState,
  getApiKeyModelAllowlistOptions,
} from '../api-key-model-access'

const access: UserModelAccess = {
  scope_mode: 'selectable_group',
  identity_scope: null,
  identity_model_ids: [],
  identity_model_ratios: {},
  identity_default_ratio: null,
  create_default_scope: 'default',
  groups: [
    {
      id: 'default',
      label: 'Default',
      ratio: 1,
      model_ids: ['gpt-5', 'claude-3'],
      model_ratios: { 'gpt-5': 0.5 },
    },
  ],
  account_model_ids: [],
  account_model_ratios: {},
  account_default_ratio: null,
  models: [
    {
      id: 'gpt-5',
      allowlist_match_key: 'gpt-5',
      vendor: null,
      supported_endpoint_types: ['openai'],
      availability_status: 'available',
    },
    {
      id: 'claude-3',
      allowlist_match_key: 'claude',
      vendor: null,
      supported_endpoint_types: ['anthropic'],
      availability_status: 'temporary_failure',
    },
  ],
}

describe('API key model access preview', () => {
  test('uses the selected group ratio and models', () => {
    const state = getApiKeyModelAccessState(access, 'default', [])
    expect(state.scope?.label).toBe('Default')
    expect(state.scopeModels.map((model) => model.id)).toEqual([
      'gpt-5',
      'claude-3',
    ])
    expect(state.effectiveModels).toHaveLength(2)
    expect(state.defaultRatio).toBe(1)
    expect(state.modelRatios).toEqual({ 'gpt-5': 0.5 })
  })

  test('applies the Base model allowlist semantics', () => {
    const state = getApiKeyModelAccessState(access, 'default', ['claude'])
    expect(state.effectiveModels.map((model) => model.id)).toEqual(['claude-3'])
    expect(getApiKeyModelAllowlistOptions(state.scopeModels)).toEqual([
      { label: 'gpt-5', value: 'gpt-5' },
      { label: 'claude-3', value: 'claude' },
    ])
  })
})
