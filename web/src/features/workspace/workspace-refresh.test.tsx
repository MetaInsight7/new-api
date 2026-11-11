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
import { act, render, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import * as userApi from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import * as workspaceApi from './api'
import { Workspace } from './workspace'

vi.mock('@/features/dashboard/components/overview/announcements-panel', () => ({
  AnnouncementsPanel: () => null,
}))
vi.mock('./components/integration/integration-panel', () => ({
  IntegrationPanel: () => null,
}))
vi.mock('./components/workspace-analytics', () => ({
  WorkspaceAnalytics: () => null,
}))
vi.mock('./components/workspace-api-access', () => ({
  WorkspaceApiAccess: () => null,
}))
vi.mock('./components/workspace-summary', () => ({
  WorkspaceSummary: () => null,
}))

function wrapper(client: QueryClient) {
  return function QueryWrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>
  }
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  vi.setSystemTime(new Date('2026-11-11T12:00:00+08:00'))
  useAuthStore.getState().auth.setUser({
    id: 17,
    username: 'workspace-user',
    role: 1,
    quota: 110,
    used_quota: 10,
    request_count: 1,
  })
})

afterEach(() => {
  useAuthStore.setState(useAuthStore.getInitialState(), true)
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test('revalidates workspace usage whenever the user re-enters the page', async () => {
  const getWorkspaceUsage = vi
    .spyOn(workspaceApi, 'getWorkspaceUsage')
    .mockResolvedValue({
      data: [],
      totalQuota: 0,
      totalCount: 0,
      totalTokens: 0,
      totalCacheReadTokens: 0,
      totalCacheCreationTokens: 0,
    })
  vi.spyOn(userApi, 'getSelf').mockResolvedValue({
    success: true,
    data: useAuthStore.getState().auth.user,
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const first = render(<Workspace />, { wrapper: wrapper(client) })

  await waitFor(() => expect(getWorkspaceUsage).toHaveBeenCalledTimes(2))
  first.unmount()
  render(<Workspace />, { wrapper: wrapper(client) })

  await waitFor(() => expect(getWorkspaceUsage).toHaveBeenCalledTimes(4), {
    timeout: 500,
  })
})

test('refreshes the cached current user whenever the workspace is re-entered', async () => {
  vi.spyOn(workspaceApi, 'getWorkspaceUsage').mockResolvedValue({
    data: [],
    totalQuota: 0,
    totalCount: 0,
    totalTokens: 0,
    totalCacheReadTokens: 0,
    totalCacheCreationTokens: 0,
  })
  let resolveSecondRefresh: (
    value: Awaited<ReturnType<typeof userApi.getSelf>>
  ) => void = () => undefined
  const secondRefresh = new Promise<
    Awaited<ReturnType<typeof userApi.getSelf>>
  >((resolve) => {
    resolveSecondRefresh = resolve
  })
  const getSelf = vi
    .spyOn(userApi, 'getSelf')
    .mockResolvedValueOnce({
      success: true,
      data: {
        id: 17,
        username: 'workspace-user',
        role: 1,
        quota: 100,
        used_quota: 20,
        request_count: 2,
      },
    })
    .mockImplementationOnce(() => secondRefresh)
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const first = render(<Workspace />, { wrapper: wrapper(client) })

  await waitFor(() =>
    expect(useAuthStore.getState().auth.user?.quota).toBe(100)
  )
  first.unmount()
  act(() => {
    useAuthStore.getState().auth.setUser({
      id: 17,
      username: 'workspace-user',
      role: 1,
      quota: 95,
      used_quota: 25,
      request_count: 2,
    })
  })
  render(<Workspace />, { wrapper: wrapper(client) })

  await waitFor(() => expect(getSelf).toHaveBeenCalledTimes(2))
  expect(useAuthStore.getState().auth.user?.quota).toBe(95)
  await act(async () => {
    resolveSecondRefresh({
      success: true,
      data: {
        id: 17,
        username: 'workspace-user',
        role: 1,
        quota: 90,
        used_quota: 30,
        request_count: 3,
      },
    })
    await secondRefresh
  })
  await waitFor(() => {
    expect(useAuthStore.getState().auth.user?.quota).toBe(90)
  })
})
