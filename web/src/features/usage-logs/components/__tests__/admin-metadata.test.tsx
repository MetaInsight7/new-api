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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test } from 'vitest'

import type { UsageLog } from '../../data/schema'
import type { LogOtherData } from '../../types'
import { DetailsDialog } from '../dialogs/details-dialog'

const queryClients: QueryClient[] = []

function makeLog(other: LogOtherData): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1,
    type: 2,
    content: '',
    username: 'user',
    token_name: 'token',
    model_name: 'claude-test',
    quota: 5000,
    prompt_tokens: 1000,
    completion_tokens: 100,
    use_time: 0,
    is_stream: false,
    channel: 1,
    channel_name: 'channel',
    token_id: 1,
    group: 'default',
    ip: '',
    other: JSON.stringify(other),
    request_id: 'req-1',
    upstream_request_id: '',
  }
}

function renderDetails(isAdmin: boolean): void {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const freshAt = Date.now() + 60_000
  queryClient.setQueryData(['status'], {}, { updatedAt: freshAt })
  queryClient.setQueryData(
    ['pricing'],
    { data: [], vendors: [] },
    { updatedAt: freshAt }
  )
  queryClients.push(queryClient)

  render(
    <QueryClientProvider client={queryClient}>
      <DetailsDialog
        log={makeLog({
          admin_info: {
            user_agent: 'test-client/1.0',
            request_body_size: 2048,
          },
          cache_tokens: 300,
          cache_creation_tokens: 200,
          claude: true,
        })}
        isAdmin={isAdmin}
        isRoot={false}
        open
        onOpenChange={() => undefined}
      />
    </QueryClientProvider>
  )
}

afterEach(() => {
  for (const queryClient of queryClients) queryClient.clear()
  queryClients.length = 0
})

describe('usage log admin request metadata', () => {
  test('shows request metadata and Claude cache hit rate to admins', () => {
    renderDetails(true)

    expect(screen.getByText('User Agent')).toBeInTheDocument()
    expect(screen.getByText('test-client/1.0')).toBeInTheDocument()
    expect(screen.getByText('Request Body Size')).toBeInTheDocument()
    expect(screen.getByText('2.0 KB')).toBeInTheDocument()
    expect(screen.getByText('Cache Hit Rate')).toBeInTheDocument()
    expect(screen.getByText('20.0%')).toBeInTheDocument()
  })

  test('keeps admin request metadata hidden from regular users', () => {
    renderDetails(false)

    expect(screen.queryByText('User Agent')).toBeNull()
    expect(screen.queryByText('test-client/1.0')).toBeNull()
    expect(screen.queryByText('Request Body Size')).toBeNull()
    expect(screen.queryByText('2.0 KB')).toBeNull()
    expect(screen.queryByText('Cache Hit Rate')).toBeNull()
    expect(screen.queryByText('20.0%')).toBeNull()
  })
})
