import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, act } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { modelAccessQueryKeys } from '@/features/available-models/hooks/use-model-access'

import { useUpdateOption } from '../use-update-option'

const { put } = vi.hoisted(() => ({ put: vi.fn() }))

vi.mock('@/lib/api', () => ({ api: { put } }))

describe('model-specific ratio save cache refresh', () => {
  beforeEach(() => put.mockReset())

  function setup() {
    const client = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })
    const cacheKey = modelAccessQueryKeys.detail(1)
    client.setQueryData(cacheKey, { scopes: [] })
    const hook = renderHook(() => useUpdateOption(), {
      wrapper: (props: { children: ReactNode }) => (
        <QueryClientProvider client={client}>
          {props.children}
        </QueryClientProvider>
      ),
    })
    return { client, cacheKey, hook }
  }

  it('invalidates the available-models cache after a successful model ratio update', async () => {
    put.mockResolvedValue({ data: { success: true, message: '' } })
    const { client, cacheKey, hook } = setup()
    await act(() =>
      hook.result.current.mutateAsync({
        key: 'GroupModelRatio',
        value: '{"vip":{"gpt-5":0.3}}',
      })
    )
    expect(client.getQueryState(cacheKey)?.isInvalidated).toBe(true)
    client.clear()
  })

  it('keeps the catalog cache unchanged when the server rejects saving', async () => {
    put.mockResolvedValue({
      data: { success: false, message: 'Invalid model ratio' },
    })
    const { client, cacheKey, hook } = setup()
    await act(async () => {
      await expect(
        hook.result.current.mutateAsync({ key: 'GroupModelRatio', value: '{}' })
      ).rejects.toThrow('Invalid model ratio')
    })
    expect(client.getQueryState(cacheKey)?.isInvalidated).toBe(false)
    client.clear()
  })

  it('does not invalidate model access for an unrelated successful option update', async () => {
    put.mockResolvedValue({ data: { success: true, message: '' } })
    const { client, cacheKey, hook } = setup()
    await act(() =>
      hook.result.current.mutateAsync({ key: 'Notice', value: '' })
    )
    expect(client.getQueryState(cacheKey)?.isInvalidated).toBe(false)
    client.clear()
  })
})
