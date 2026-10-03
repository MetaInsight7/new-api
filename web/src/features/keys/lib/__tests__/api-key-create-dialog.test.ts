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
  getCreateDialogDeepLinkAction,
  getCreateDialogRequestKey,
  shouldApplyResolvedCreateGroup,
} from '../api-key-create-dialog'

function buildAccess(): UserModelAccess {
  return {
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
        model_ids: [],
        model_ratios: {},
      },
      {
        id: 'premium',
        label: 'Premium',
        ratio: 2,
        model_ids: [],
        model_ratios: {},
      },
    ],
    account_model_ids: [],
    account_model_ratios: {},
    account_default_ratio: null,
    models: [],
  }
}

describe('API key create dialog deep links', () => {
  test('opens after access data resolves and safely falls back', () => {
    expect(
      getCreateDialogDeepLinkAction(true, buildAccess(), false, 'premium')
    ).toEqual({ shouldOpen: true, resolvedGroup: 'premium' })
    expect(
      getCreateDialogDeepLinkAction(true, buildAccess(), false, 'removed')
    ).toEqual({ shouldOpen: true, resolvedGroup: 'default' })
  })

  test('opens on a model access error so the dialog can offer retry', () => {
    expect(getCreateDialogDeepLinkAction(true, undefined, false)).toEqual({
      shouldOpen: false,
    })
    expect(getCreateDialogDeepLinkAction(true, undefined, true)).toEqual({
      shouldOpen: true,
    })
  })

  test('changes request keys when the requested group changes', () => {
    expect(getCreateDialogRequestKey(true, 'default')).toBe('create:default')
    expect(getCreateDialogRequestKey(true, 'premium')).toBe('create:premium')
    expect(getCreateDialogRequestKey(false, 'premium')).toBeNull()
  })

  test('does not overwrite a group selected by the user', () => {
    const options = {
      access: buildAccess(),
      initialized: true,
      isUpdate: false,
      open: true,
    }
    expect(
      shouldApplyResolvedCreateGroup({ ...options, groupDirty: false })
    ).toBe(true)
    expect(
      shouldApplyResolvedCreateGroup({ ...options, groupDirty: true })
    ).toBe(false)
  })
})
