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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { IntegrationPanel } from '../integration-panel'

const mocks = vi.hoisted(() => ({
  getApiKeys: vi.fn(),
  getPricing: vi.fn(),
  getUserModels: vi.fn(),
}))

vi.mock('@/features/keys/api', () => ({ getApiKeys: mocks.getApiKeys }))
vi.mock('@/features/pricing/api', () => ({ getPricing: mocks.getPricing }))
vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: { server_address: 'https://api.example' } }),
}))
vi.mock('@/lib/api', () => ({ getUserModels: mocks.getUserModels }))
vi.mock('../integration-cards', () => ({
  IntegrationCards: (props: { onSelect: (id: 'api') => void }) => (
    <button type='button' onClick={() => props.onSelect('api')}>
      Open integration
    </button>
  ),
}))
vi.mock('../integration-dialog', () => ({
  IntegrationDialog: (props: { keysError?: string; modelsError?: string }) => (
    <div>
      <span>{props.keysError}</span>
      <span>{props.modelsError}</span>
    </div>
  ),
}))
vi.mock('../use-resolved-api-keys', () => ({
  useResolvedApiKeys: () => ({
    resolvedKeys: {},
    loadingKeys: {},
    resolveKey: vi.fn(),
  }),
}))

test('business failures enter query error state and render their messages', async () => {
  mocks.getApiKeys.mockResolvedValue({
    success: false,
    message: 'API keys unavailable',
  })
  mocks.getUserModels.mockResolvedValue({
    success: false,
    message: 'Models unavailable',
  })
  mocks.getPricing.mockResolvedValue({ success: true, data: [] })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <IntegrationPanel />
    </QueryClientProvider>
  )

  fireEvent.click(screen.getByRole('button', { name: 'Open integration' }))

  await waitFor(() => {
    expect(
      client.getQueryState(['workspace', 'integration', 'api-keys'])?.status
    ).toBe('error')
    expect(
      client.getQueryState(['workspace', 'integration', 'models'])?.status
    ).toBe('error')
  })
  expect(screen.getByText('API keys unavailable')).toBeVisible()
  expect(screen.getByText('Models unavailable')).toBeVisible()
})
