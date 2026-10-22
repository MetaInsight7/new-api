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
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import {
  canAccessChannelLoadMonitor,
  channelLoadMonitorSearchSchema,
} from '@/routes/_authenticated/channel-load-monitor'
import { useAuthStore } from '@/stores/auth-store'

import { ChannelLoadMonitor } from '..'
import { channelSchema, type Channel } from '../../channels/types'
import type { ChannelLoadMonitorData, ChannelLoadMonitorItem } from '../types'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  initAsync: false,
})

let client: QueryClient
const originalUser = useAuthStore.getState().auth.user

function channel(
  id: number,
  name: string,
  maxConcurrency: number,
  group = 'default'
): Channel {
  return channelSchema.parse({
    id,
    type: 1,
    key: '',
    name,
    status: 1,
    group,
    priority: 10,
    weight: 20,
    max_concurrency: maxConcurrency,
    created_time: 1,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
  })
}

function monitorItem(
  value: Channel,
  active: number,
  waiting = 0,
  coolingDown = false
): ChannelLoadMonitorItem {
  return {
    channel: value,
    concurrency:
      value.max_concurrency > 0
        ? {
            channel_id: value.id,
            max_concurrency: value.max_concurrency,
            active,
            waiting,
            cooling_down: coolingDown,
          }
        : null,
  }
}

function monitorData(
  items: ChannelLoadMonitorItem[],
  overrides: Partial<ChannelLoadMonitorData> = {}
): ChannelLoadMonitorData {
  return {
    items,
    total: items.length,
    page: 1,
    page_size: 20,
    type_counts: { 1: items.length },
    groups: [
      ...new Set(items.flatMap((item) => item.channel.group.split(','))),
    ],
    summary: {
      limited_rpm: 12,
      active: 1,
      capacity: 3,
      waiting: 0,
      cooling_down: 0,
    },
    snapshot_at: 1_800_000_000_000,
    ...overrides,
  }
}

async function renderMonitor(initialEntry = '/channel-load-monitor/') {
  const root = createRootRoute()
  const authenticated = createRoute({
    getParentRoute: () => root,
    id: '_authenticated',
  })
  const monitorRoute = createRoute({
    getParentRoute: () => authenticated,
    path: 'channel-load-monitor/',
    validateSearch: channelLoadMonitorSearchSchema,
    component: ChannelLoadMonitor,
  })
  const router = createRouter({
    routeTree: root.addChildren([authenticated.addChildren([monitorRoute])]),
    history: createMemoryHistory({ initialEntries: [initialEntry] }),
  })

  await router.load()
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </I18nextProvider>
  )
  return router
}

function monitorRequestParams(
  get: ReturnType<typeof vi.spyOn>
): Array<Record<string, unknown>> {
  return get.mock.calls.flatMap((call: unknown[]) => {
    if (call[0] !== '/api/channel/concurrency/monitor') return []
    const config = call[1] as { params?: Record<string, unknown> } | undefined
    return config?.params ? [config.params] : []
  })
}

beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  useAuthStore.getState().auth.setUser({
    id: 2,
    username: 'channel-reader',
    role: ROLE.ADMIN,
    permissions: {
      admin_permissions: { channel: { read: true, write: false } },
    },
  })
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
})

afterEach(() => {
  cleanup()
  client.clear()
  useAuthStore.getState().auth.setUser(originalUser)
  Object.defineProperty(document, 'visibilityState', {
    configurable: true,
    value: 'visible',
  })
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('channel load monitor access', () => {
  test('requires both an administrator role and channel read permission', () => {
    expect(
      canAccessChannelLoadMonitor({
        id: 1,
        username: 'reader',
        role: ROLE.ADMIN,
        permissions: {
          admin_permissions: { channel: { read: true } },
        },
      })
    ).toBe(true)
    expect(
      canAccessChannelLoadMonitor({
        id: 2,
        username: 'restricted-admin',
        role: ROLE.ADMIN,
        permissions: {
          admin_permissions: { channel: { read: false } },
        },
      })
    ).toBe(false)
    expect(
      canAccessChannelLoadMonitor({
        id: 3,
        username: 'ordinary-user',
        role: ROLE.USER,
        permissions: {
          admin_permissions: { channel: { read: true } },
        },
      })
    ).toBe(false)
  })
})

test('renders full-filter summary and drives server pagination and URL filters', async () => {
  const item = monitorItem(channel(7, 'alpha-channel', 3), 3, 2, true)
  const response = monitorData([item], {
    total: 41,
    summary: {
      limited_rpm: 37,
      active: 5,
      capacity: 7,
      waiting: 2,
      cooling_down: 1,
    },
  })
  const get = vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/channel/concurrency/monitor') {
      return { data: { success: true, data: response } }
    }
    throw new Error(`Unexpected URL: ${url}`)
  })
  const router = await renderMonitor()
  const user = userEvent.setup()

  await screen.findByRole('table')
  const table = screen.getByRole('table')
  const limitedCard = screen
    .getByText('Limited-channel RPM')
    .closest('[data-slot="card"]') as HTMLElement
  expect(within(limitedCard).getByText('37')).toBeVisible()
  expect(
    within(limitedCard).getByText('Only channels with concurrency limits')
  ).toBeVisible()
  expect(screen.queryByText(/Unlimited-channel RPM/)).not.toBeInTheDocument()
  expect(within(table).getByText('OpenAI')).toBeVisible()
  expect(within(table).getByText('Enabled')).toBeVisible()
  expect(within(table).getByText('default')).toBeVisible()
  expect(within(table).getByRole('button', { name: '10' })).toBeDisabled()
  expect(
    within(table).getByRole('progressbar', {
      name: 'alpha-channel Slot usage',
    })
  ).toHaveAttribute('aria-valuenow', '100')
  expect(
    within(table).getByText('alpha-channel').closest('.w-32')
  ).toBeVisible()
  for (const filterName of ['Status', 'Type', 'Group']) {
    expect(screen.getByRole('button', { name: filterName })).toBeVisible()
  }
  expect(
    screen.getByRole('button', {
      name: /Priority.*Priority: high to low/,
    })
  ).toBeVisible()
  const filterCard = screen
    .getByRole('textbox', { name: 'Search channels' })
    .closest('[data-slot="card"]') as HTMLElement
  expect(
    within(filterCard).getByText('Auto-refreshing every 10s')
  ).toBeVisible()
  expect(within(filterCard).getByText(/^Last updated:/)).toBeVisible()
  expect(screen.getAllByText('Auto-refreshing every 10s')).toHaveLength(1)
  expect(screen.getAllByText('alpha-channel')).toHaveLength(2)
  expect(
    screen.queryByRole('button', { name: 'Edit maximum concurrency: 3' })
  ).not.toBeInTheDocument()
  expect(monitorRequestParams(get)[0]).toEqual(
    expect.objectContaining({
      p: 1,
      page_size: 20,
      load_state: 'all',
      sort_by: 'priority',
      sort_order: 'desc',
    })
  )

  await user.click(screen.getByRole('button', { name: 'Type' }))
  expect(screen.getByPlaceholderText('Type')).toBeVisible()
  expect(screen.getByRole('option', { name: /OpenAI/ })).toBeVisible()
  expect(
    screen.queryByRole('option', { name: /Azure/ })
  ).not.toBeInTheDocument()
  await user.keyboard('{Escape}')

  await user.click(screen.getByRole('button', { name: 'Group' }))
  expect(screen.getByRole('option', { name: 'default' })).toBeVisible()
  expect(screen.queryByRole('option', { name: 'beta' })).not.toBeInTheDocument()
  await user.keyboard('{Escape}')

  await user.click(screen.getByRole('button', { name: 'Status' }))
  expect(screen.getByPlaceholderText('Status')).toBeVisible()
  await user.click(screen.getByRole('option', { name: 'Disabled' }))
  await waitFor(() =>
    expect(
      monitorRequestParams(get).some((params) => params.status === 'disabled')
    ).toBe(true)
  )
  expect(router.state.location.search).toMatchObject({ status: 'disabled' })
  await user.keyboard('{Escape}')

  await user.click(screen.getByRole('button', { name: 'Go to next page' }))
  await waitFor(() =>
    expect(
      monitorRequestParams(get).some(
        (params) => params.p === 2 && params.page_size === 20
      )
    ).toBe(true)
  )
  expect(router.state.location.search).toMatchObject({ page: 2, pageSize: 20 })

  const search = screen.getByRole('textbox', { name: 'Search channels' })
  await user.type(search, 'beta')
  await user.click(screen.getByRole('button', { name: 'Queued' }))
  await waitFor(() =>
    expect(
      monitorRequestParams(get).some(
        (params) =>
          params.keyword === 'beta' &&
          params.load_state === 'queued' &&
          params.p === 1
      )
    ).toBe(true)
  )
  expect(router.state.location.search).toMatchObject({
    filter: 'beta',
    loadState: 'queued',
    page: 1,
  })
})

test('uses ten rows and compact cards on a mobile viewport', async () => {
  const originalMatchMedia = window.matchMedia
  vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
    ...originalMatchMedia(query),
    matches:
      query === '(max-width: 640px)' || originalMatchMedia(query).matches,
  }))
  const item = monitorItem(channel(8, 'mobile-channel', 2), 1)
  const get = vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/channel/concurrency/monitor') {
      return {
        data: {
          success: true,
          data: monitorData([item], { page_size: 10 }),
        },
      }
    }
    throw new Error(`Unexpected URL: ${url}`)
  })

  await renderMonitor()
  await screen.findByRole('navigation', { name: 'Page' })
  expect(monitorRequestParams(get)[0]).toEqual(
    expect.objectContaining({ page_size: 10 })
  )
  const mobileName = screen
    .getAllByText('mobile-channel')
    .find((element) => element.closest('[data-slot="card"]'))
  const mobileCard = mobileName?.closest('[data-slot="card"]') as HTMLElement
  expect(mobileCard.parentElement).toHaveClass('sm:hidden')
  expect(screen.getByRole('table').closest('.sm\\:block')).toHaveClass('hidden')
})

test('pauses while hidden, refreshes immediately when visible, and keeps stale data', async () => {
  vi.useFakeTimers()
  const item = monitorItem(channel(9, 'retained-channel', 4), 2)
  let monitorCallCount = 0
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/channel/concurrency/monitor') {
      monitorCallCount++
      if (monitorCallCount === 1) {
        return {
          data: { success: true, data: monitorData([item]) },
        }
      }
      throw new Error('snapshot unavailable')
    }
    throw new Error(`Unexpected URL: ${url}`)
  })

  await renderMonitor()
  await act(async () => vi.advanceTimersByTimeAsync(1))
  expect(screen.getAllByText('retained-channel')).toHaveLength(2)
  expect(monitorCallCount).toBe(1)

  Object.defineProperty(document, 'visibilityState', {
    configurable: true,
    value: 'hidden',
  })
  fireEvent(window, new Event('visibilitychange'))
  await act(async () => vi.advanceTimersByTimeAsync(30_000))
  expect(monitorCallCount).toBe(1)

  Object.defineProperty(document, 'visibilityState', {
    configurable: true,
    value: 'visible',
  })
  fireEvent(window, new Event('visibilitychange'))
  await act(async () => vi.advanceTimersByTimeAsync(1))
  expect(monitorCallCount).toBe(2)
  expect(screen.getByRole('status')).toHaveTextContent(
    'Refresh failed. Showing the last successful snapshot'
  )
  expect(screen.getAllByText('retained-channel')).toHaveLength(2)
})
