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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import * as modelAccessApi from '@/features/available-models/api'
import { modelAccessQueryKeys } from '@/features/available-models/hooks/use-model-access'
import type { UserModelAccess } from '@/features/available-models/types'
import { useAuthStore } from '@/stores/auth-store'

import { ApiKeysProvider, useApiKeys } from '../api-keys-provider'

const modelAccess: UserModelAccess = {
  scope_mode: 'selectable_group',
  identity_scope: 'default',
  identity_model_ids: [],
  identity_model_ratios: {},
  identity_default_ratio: 1,
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
  account_default_ratio: 1,
  models: [],
}

function wrapper(client: QueryClient) {
  return function QueryWrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>
  }
}

function ApiKeyEditorControls() {
  const { open, setOpen, initialCreateGroup } = useApiKeys()

  return (
    <>
      <p>Editor: {open ?? 'closed'}</p>
      <p>Initial group: {initialCreateGroup ?? 'none'}</p>
      <button type='button' onClick={() => setOpen('create')}>
        Open editor
      </button>
      <button type='button' onClick={() => setOpen(null)}>
        Close editor
      </button>
    </>
  )
}

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: 10 * 1000,
      },
    },
  })
}

beforeEach(() => {
  useAuthStore.getState().auth.setUser({
    id: 23,
    username: 'keys-user',
    role: 1,
  })
})

afterEach(() => {
  useAuthStore.setState(useAuthStore.getInitialState(), true)
  vi.restoreAllMocks()
})

test('loads model access only when the API key editor needs it', async () => {
  const getUserModelAccess = vi
    .spyOn(modelAccessApi, 'getUserModelAccess')
    .mockResolvedValue(modelAccess)
  const client = createQueryClient()
  const user = userEvent.setup()

  render(
    <ApiKeysProvider>
      <ApiKeyEditorControls />
    </ApiKeysProvider>,
    { wrapper: wrapper(client) }
  )

  expect(screen.getByText('Editor: closed')).toBeVisible()
  expect(getUserModelAccess).not.toHaveBeenCalled()

  await user.click(screen.getByRole('button', { name: 'Open editor' }))
  await waitFor(() => expect(getUserModelAccess).toHaveBeenCalledTimes(1))

  await user.click(screen.getByRole('button', { name: 'Close editor' }))
  await user.click(screen.getByRole('button', { name: 'Open editor' }))
  expect(getUserModelAccess).toHaveBeenCalledTimes(1)

  await user.click(screen.getByRole('button', { name: 'Close editor' }))
  client.setQueryData(modelAccessQueryKeys.detail(23), modelAccess, {
    updatedAt: Date.now() - 10_001,
  })
  await user.click(screen.getByRole('button', { name: 'Open editor' }))
  await waitFor(() => expect(getUserModelAccess).toHaveBeenCalledTimes(2))
})

test('loads model access before opening a deep-linked create editor', async () => {
  const getUserModelAccess = vi
    .spyOn(modelAccessApi, 'getUserModelAccess')
    .mockResolvedValue(modelAccess)
  const client = createQueryClient()

  render(
    <ApiKeysProvider createDialogRequested requestedCreateGroup='premium'>
      <ApiKeyEditorControls />
    </ApiKeysProvider>,
    { wrapper: wrapper(client) }
  )

  await waitFor(() => expect(getUserModelAccess).toHaveBeenCalledTimes(1))
  await waitFor(() => expect(screen.getByText('Editor: create')).toBeVisible())
  expect(screen.getByText('Initial group: premium')).toBeVisible()
})
