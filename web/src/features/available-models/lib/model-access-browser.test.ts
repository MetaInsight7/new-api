import type { TFunction } from 'i18next'
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
  ALL_MODEL_VENDORS,
  filterModelAccessModels,
  getModelEndpointLabel,
  getModelVendorFilters,
  normalizeModelAvailabilityStatus,
  resolveModelAccessScope,
} from './model-access-browser'

const access: UserModelAccess = {
  scope_mode: 'selectable_group',
  identity_scope: 'default',
  identity_model_ids: [],
  create_default_scope: null,
  groups: [{ id: 'default', label: 'Default', ratio: 1, model_ids: ['gpt'] }],
  account_model_ids: [],
  models: [
    {
      id: 'gpt',
      allowlist_match_key: 'gpt',
      vendor: { id: 1, name: 'OpenAI' },
      supported_endpoint_types: ['openai'],
      availability_status: 'available',
    },
    {
      id: 'local-model',
      allowlist_match_key: 'local-model',
      vendor: null,
      supported_endpoint_types: ['anthropic'],
      availability_status: 'unknown',
    },
  ],
}

describe('available model browser filtering', () => {
  test('filters by model name and vendor', () => {
    const filters = getModelVendorFilters(access.models)
    expect(filters.map((filter) => filter.value)).toEqual([
      ALL_MODEL_VENDORS,
      'vendor:openai',
      'unlabelled',
    ])
    expect(
      filterModelAccessModels(access.models, 'gpt', ALL_MODEL_VENDORS)
    ).toHaveLength(1)
    expect(filterModelAccessModels(access.models, '', 'vendor:openai')).toEqual(
      [access.models[0]]
    )
  })

  test('normalizes unknown status and endpoint labels', () => {
    expect(normalizeModelAvailabilityStatus('unknown')).toBe('unknown_failure')
    const translate = ((key: string) => key) as unknown as TFunction
    expect(getModelEndpointLabel('openai-responses', translate)).toBe(
      'OpenAI Compatible'
    )
    expect(resolveModelAccessScope(access, 'missing')).toBe('default')
  })
})
