/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { afterEach, describe, expect, it } from 'vitest'

import { ROLE } from '@/lib/roles'
import { canAccessSensitiveWordMonitor } from '@/routes/_authenticated/sensitive-word-monitor'
import { useAuthStore } from '@/stores/auth-store'

import { setSensitiveWordManager } from './test-utils'

afterEach(() => {
  useAuthStore.getState().auth.reset()
})

describe('sensitive-word monitor access', () => {
  it('requires an administrator with sensitive-word management permission', () => {
    setSensitiveWordManager()
    expect(
      canAccessSensitiveWordMonitor(useAuthStore.getState().auth.user)
    ).toBe(true)
    expect(
      canAccessSensitiveWordMonitor({
        id: 2,
        username: 'admin-without-permission',
        role: ROLE.ADMIN,
        permissions: {
          admin_permissions: { sensitive_word: { manage: false } },
        },
      })
    ).toBe(false)
    expect(
      canAccessSensitiveWordMonitor({
        id: 3,
        username: 'ordinary-user',
        role: ROLE.USER,
        permissions: {
          admin_permissions: { sensitive_word: { manage: true } },
        },
      })
    ).toBe(false)
  })
})
