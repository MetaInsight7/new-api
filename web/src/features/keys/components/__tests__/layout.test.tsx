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
import { afterEach, describe, expect, it } from 'vitest'

import { ApiKeysMutateDrawer } from '../api-keys-mutate-drawer'
import { ApiKeysProvider } from '../api-keys-provider'

let queryClient: QueryClient | null = null

function renderCreateDialog() {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const freshAt = Date.now() + 60_000
  queryClient.setQueryData(
    ['status'],
    { default_use_auto_group: false },
    { updatedAt: freshAt }
  )
  queryClient.setQueryData(
    ['user-models'],
    { success: true, data: [] },
    { updatedAt: freshAt }
  )
  queryClient.setQueryData(
    ['user-groups'],
    {
      success: true,
      data: { default: { desc: 'Standard access', ratio: 1 } },
    },
    { updatedAt: freshAt }
  )
  queryClient.setQueryData(
    ['token-auto-groups'],
    { success: true, data: { groups: [], max_count: 3 } },
    { updatedAt: freshAt }
  )

  render(
    <QueryClientProvider client={queryClient}>
      <ApiKeysProvider>
        <ApiKeysMutateDrawer open onOpenChange={() => undefined} />
      </ApiKeysProvider>
    </QueryClientProvider>
  )
}

afterEach(() => {
  queryClient?.clear()
  queryClient = null
})

describe('API key editor layout', () => {
  it('keeps the form narrower than the model preview on desktop', async () => {
    renderCreateDialog()
    await waitFor(() => {
      expect(
        screen.getByRole('dialog', { name: 'Create API Key' })
      ).toBeVisible()
    })

    const layout = document.querySelector('[data-slot="api-key-editor-layout"]')
    expect(layout).toHaveClass(
      'lg:grid-cols-[minmax(0,0.92fr)_minmax(0,1.08fr)]'
    )
  })
})
