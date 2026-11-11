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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import * as modelAccessApi from './api'
import { AvailableModels } from './available-models-page'
import type { UserModelAccess } from './types'

vi.mock('@/components/OperationDocumentationLink', () => ({
  OperationDocumentationLink: () => null,
}))
vi.mock('./components/model-access-browser', () => ({
  ModelAccessBrowser: () => <div>available models loaded</div>,
}))

function wrapper(client: QueryClient) {
  return function QueryWrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>
  }
}

beforeEach(() => {
  useAuthStore.getState().auth.setUser({
    id: 23,
    username: 'models-user',
    role: 1,
  })
})

afterEach(() => {
  useAuthStore.setState(useAuthStore.getInitialState(), true)
  vi.restoreAllMocks()
})

test('revalidates available models whenever the user re-enters the page', async () => {
  const getUserModelAccess = vi
    .spyOn(modelAccessApi, 'getUserModelAccess')
    .mockResolvedValue({} as UserModelAccess)
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const first = render(<AvailableModels />, { wrapper: wrapper(client) })

  await waitFor(() => expect(getUserModelAccess).toHaveBeenCalledTimes(1))
  first.unmount()
  render(<AvailableModels />, { wrapper: wrapper(client) })

  await waitFor(() => expect(getUserModelAccess).toHaveBeenCalledTimes(2), {
    timeout: 500,
  })
})
