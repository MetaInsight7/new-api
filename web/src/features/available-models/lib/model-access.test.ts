/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import { describe, expect, test } from 'vitest'

import type { UserModelAccess } from '../types'
import {
  getAccountModels,
  getScopeModels,
  getUnavailableScopeModels,
  resolveCreateScope,
} from './model-access'

const access: UserModelAccess = {
  scope_mode: 'selectable_group',
  identity_scope: 'default',
  identity_model_ids: ['identity-model'],
  create_default_scope: 'vip',
  groups: [
    {
      id: 'vip',
      label: 'VIP',
      ratio: 0.8,
      model_ids: ['callable-model', 'unavailable-model'],
      model_ratios: {},
    },
  ],
  account_model_ids: [],
  account_model_ratios: {},
  account_default_ratio: null,
  identity_model_ratios: {},
  identity_default_ratio: 1,
  models: [
    {
      id: 'callable-model',
      allowlist_match_key: 'callable-model',
      vendor: null,
      supported_endpoint_types: ['openai'],
      availability_status: 'available',
    },
    {
      id: 'unavailable-model',
      allowlist_match_key: 'unavailable-model',
      vendor: null,
      supported_endpoint_types: ['openai'],
      availability_status: 'official_unsupported',
    },
  ],
}

describe('available model access resolution', () => {
  test('prefers a valid requested scope and then the configured default', () => {
    expect(resolveCreateScope(access, 'vip')).toBe('vip')
    expect(resolveCreateScope(access, 'missing')).toBe('vip')
    expect(resolveCreateScope({ ...access, create_default_scope: null })).toBe(
      null
    )
  })

  test('separates callable and unavailable models per scope', () => {
    expect(getScopeModels(access, 'vip').map((model) => model.id)).toEqual([
      'callable-model',
    ])
    expect(
      getUnavailableScopeModels(access, 'vip').map((model) => model.id)
    ).toEqual(['unavailable-model'])
  })

  test('uses fixed account models without requiring a scope', () => {
    const fixedAccess: UserModelAccess = {
      ...access,
      scope_mode: 'fixed_account',
      account_model_ids: ['callable-model'],
      groups: [],
    }
    expect(getAccountModels(fixedAccess).map((model) => model.id)).toEqual([
      'callable-model',
    ])
  })
})
