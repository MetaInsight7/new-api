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

import { ROLE } from '@/lib/roles'

import { resolveHomeRedirect } from '../home-redirect'

describe('home redirect', () => {
  test('routes regular users to the workspace', async () => {
    const result = await resolveHomeRedirect(async () => ({
      success: true,
      data: { id: 1, username: 'user', role: ROLE.USER },
    }))

    expect(result).toEqual({
      to: '/workspace',
      user: { id: 1, username: 'user', role: ROLE.USER },
    })
  })

  test('routes administrators to the dashboard', async () => {
    const result = await resolveHomeRedirect(async () => ({
      success: true,
      data: { id: 2, username: 'admin', role: ROLE.ADMIN },
    }))

    expect(result.to).toBe('/dashboard')
    expect(result.user?.id).toBe(2)
  })

  test('routes unavailable sessions to sign-in', async () => {
    await expect(
      resolveHomeRedirect(async () => ({ success: false }))
    ).resolves.toEqual({ to: '/sign-in', user: null })

    await expect(
      resolveHomeRedirect(async () => {
        throw new Error('network unavailable')
      })
    ).resolves.toEqual({ to: '/sign-in', user: null })
  })
})
