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
import { act, fireEvent, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { toast } from 'sonner'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { createAppQueryClient } from '@/lib/query-client'

import { aggregateChannelsByTag } from '../../lib'
import { channelSchema, type Channel } from '../../types'
import { useChannelConcurrencySnapshots } from '../use-channel-concurrency-snapshots'

function channel(id: number, maxConcurrency: number): Channel {
  return channelSchema.parse({
    id,
    type: 1,
    key: '',
    name: `channel-${id}`,
    status: 1,
    max_concurrency: maxConcurrency,
    created_time: 1,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
  })
}

function wrapper(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>
  }
}

afterEach(() => {
  Object.defineProperty(document, 'visibilityState', {
    configurable: true,
    value: 'visible',
  })
  fireEvent(document, new Event('visibilitychange'))
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test('requests the current page limited channels immediately with a stable sorted set', async () => {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const get = vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: [
        {
          channel_id: 9,
          max_concurrency: 2,
          active: 1,
          waiting: 0,
          cooling_down: false,
        },
      ],
    },
  })
  const limitedNine = channel(9, 2)
  const limitedThree = channel(3, 1)
  const channels = aggregateChannelsByTag([
    limitedNine,
    limitedThree,
    limitedNine,
    channel(1, 0),
  ])

  const hook = renderHook(({ rows }) => useChannelConcurrencySnapshots(rows), {
    initialProps: { rows: channels },
    wrapper: wrapper(client),
  })

  await waitFor(() => expect(get).toHaveBeenCalledTimes(1))
  expect(get).toHaveBeenCalledWith('/api/channel/concurrency', {
    params: { ids: '3,9' },
    signal: expect.any(AbortSignal),
    disableDuplicate: true,
    skipBusinessError: true,
    skipErrorHandler: true,
  })
  await waitFor(() =>
    expect(hook.result.current.statusById.get(9)?.active).toBe(1)
  )

  hook.rerender({ rows: [limitedThree, limitedNine] })
  expect(get).toHaveBeenCalledTimes(1)
})

test('polls every ten visible seconds, pauses while hidden, and stops after unmount', async () => {
  vi.useFakeTimers()
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const get = vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: [] },
  })
  const hook = renderHook(
    () => useChannelConcurrencySnapshots([channel(7, 2)]),
    {
      wrapper: wrapper(client),
    }
  )

  await act(async () => vi.advanceTimersByTimeAsync(1))
  expect(get).toHaveBeenCalledTimes(1)
  await act(async () => vi.advanceTimersByTimeAsync(9_999))
  expect(get).toHaveBeenCalledTimes(2)

  Object.defineProperty(document, 'visibilityState', {
    configurable: true,
    value: 'hidden',
  })
  fireEvent(document, new Event('visibilitychange'))
  await act(async () => vi.advanceTimersByTimeAsync(30_000))
  expect(get).toHaveBeenCalledTimes(2)

  Object.defineProperty(document, 'visibilityState', {
    configurable: true,
    value: 'visible',
  })
  fireEvent(document, new Event('visibilitychange'))
  await act(async () => vi.advanceTimersByTimeAsync(9_999))
  expect(get).toHaveBeenCalledTimes(2)
  await act(async () => vi.advanceTimersByTimeAsync(1))
  expect(get).toHaveBeenCalledTimes(3)

  hook.unmount()
  await act(async () => vi.advanceTimersByTimeAsync(20_000))
  expect(get).toHaveBeenCalledTimes(3)
})

test.each(['network', 'http', 'business'] as const)(
  'keeps the snapshot unavailable without a toast after an initial %s failure',
  async (failure) => {
    const client = createAppQueryClient()
    const errorToast = vi.spyOn(toast, 'error')
    const get = vi.spyOn(api, 'get')
    if (failure === 'business') {
      get.mockResolvedValue({
        data: { success: false, message: 'snapshot unavailable', data: [] },
      })
    } else {
      const error = new Error(`${failure} failure`) as Error & {
        isAxiosError?: boolean
        response?: { status: number; data: unknown }
      }
      if (failure === 'http') {
        error.isAxiosError = true
        error.response = { status: 503, data: { success: false } }
      }
      get.mockRejectedValue(error)
    }

    const hook = renderHook(
      () => useChannelConcurrencySnapshots([channel(7, 2)]),
      { wrapper: wrapper(client) }
    )

    await waitFor(() => expect(hook.result.current.isError).toBe(true))
    expect(hook.result.current.statusById.size).toBe(0)
    expect(get).toHaveBeenCalledTimes(1)
    expect(errorToast).not.toHaveBeenCalled()
  }
)

test('retains the last successful snapshot through refresh failures and replaces it later', async () => {
  vi.useFakeTimers()
  const client = createAppQueryClient()
  const get = vi
    .spyOn(api, 'get')
    .mockResolvedValueOnce({
      data: {
        success: true,
        data: [
          {
            channel_id: 7,
            max_concurrency: 2,
            active: 1,
            waiting: 0,
            cooling_down: false,
          },
        ],
      },
    })
    .mockRejectedValueOnce(new Error('network failure'))
    .mockResolvedValueOnce({
      data: { success: false, message: 'business failure', data: [] },
    })
    .mockResolvedValueOnce({
      data: {
        success: true,
        data: [
          {
            channel_id: 7,
            max_concurrency: 2,
            active: 2,
            waiting: 1,
            cooling_down: true,
          },
        ],
      },
    })

  const hook = renderHook(
    () => useChannelConcurrencySnapshots([channel(7, 2)]),
    { wrapper: wrapper(client) }
  )

  await act(async () => vi.advanceTimersByTimeAsync(1))
  expect(hook.result.current.statusById.get(7)?.active).toBe(1)
  await act(async () => vi.advanceTimersByTimeAsync(9_999))
  expect(hook.result.current.statusById.get(7)?.active).toBe(1)
  await act(async () => vi.advanceTimersByTimeAsync(10_000))
  expect(hook.result.current.statusById.get(7)?.active).toBe(1)
  await act(async () => vi.advanceTimersByTimeAsync(10_000))
  expect(get).toHaveBeenCalledTimes(4)
  await act(async () => vi.advanceTimersByTimeAsync(1))
  expect(hook.result.current.statusById.get(7)?.active).toBe(2)
  expect(hook.result.current.statusById.get(7)?.waiting).toBe(1)
})

test('cancels the old collection and never carries its mapping into a new or empty set', async () => {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  let resolveOld!: (value: {
    data: { success: boolean; data: unknown[] }
  }) => void
  const oldResponse = new Promise<{
    data: { success: boolean; data: unknown[] }
  }>((resolve) => {
    resolveOld = resolve
  })
  let oldSignal: { readonly aborted: boolean } | undefined
  const get = vi.spyOn(api, 'get').mockImplementation((_url, config) => {
    if (config?.params?.ids === '7') {
      oldSignal = config.signal
      return oldResponse
    }
    return Promise.resolve({
      data: {
        success: true,
        data: [
          {
            channel_id: 8,
            max_concurrency: 3,
            active: 2,
            waiting: 0,
            cooling_down: false,
          },
        ],
      },
    })
  })
  const hook = renderHook(({ rows }) => useChannelConcurrencySnapshots(rows), {
    initialProps: { rows: [channel(7, 2)] },
    wrapper: wrapper(client),
  })
  await waitFor(() => expect(get).toHaveBeenCalledTimes(1))

  hook.rerender({ rows: [channel(8, 3)] })
  await waitFor(() => expect(get).toHaveBeenCalledTimes(2))
  expect(oldSignal?.aborted).toBe(true)
  expect(hook.result.current.statusById.has(7)).toBe(false)
  await waitFor(() =>
    expect(hook.result.current.statusById.get(8)?.active).toBe(2)
  )

  resolveOld({ data: { success: true, data: [] } })
  await Promise.resolve()
  expect(hook.result.current.statusById.get(8)?.active).toBe(2)

  hook.rerender({ rows: [] })
  expect(hook.result.current.statusById.size).toBe(0)
  expect(get).toHaveBeenCalledTimes(2)
})
