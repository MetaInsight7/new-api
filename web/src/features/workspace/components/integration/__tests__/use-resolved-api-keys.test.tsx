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

For commercial licensing, please contact support@quantumnous.com
*/
import { QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { toast } from 'sonner'
import { afterEach, expect, test, vi } from 'vitest'

import * as keysApi from '@/features/keys/api'
import { createAppQueryClient } from '@/lib/query-client'
import { useAuthStore } from '@/stores/auth-store'

import { useResolvedApiKeys } from '../use-resolved-api-keys'

afterEach(() => {
  useAuthStore.setState(useAuthStore.getInitialState(), true)
  vi.restoreAllMocks()
})

test('a locally handled key-resolution failure emits exactly one toast', async () => {
  useAuthStore.getState().auth.setUser({
    id: 17,
    username: 'integration-user',
    role: 1,
  })
  vi.spyOn(keysApi, 'fetchTokenKey').mockRejectedValue(
    new Error('key unavailable')
  )
  const errorToast = vi.spyOn(toast, 'error').mockImplementation(() => 'toast')
  const client = createAppQueryClient()
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
  const hook = renderHook(() => useResolvedApiKeys(null, true), { wrapper })

  let result: string | null = 'pending'
  await act(async () => {
    result = await hook.result.current.resolveKey(9)
  })

  expect(result).toBeNull()
  expect(errorToast).toHaveBeenCalledTimes(1)
})
