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
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import type { ChannelConcurrencyStatus } from '../../api'
import { channelSchema, type Channel } from '../../types'
import { ChannelsProvider } from '../channels-provider'
import { ChannelsTable } from '../channels-table'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  initAsync: false,
})

let client: QueryClient
const originalUser = useAuthStore.getState().auth.user

type SnapshotResponse = {
  data: { success: boolean; data: ChannelConcurrencyStatus[] }
}
type SnapshotSource =
  | SnapshotResponse
  | Promise<SnapshotResponse>
  | (() => SnapshotResponse | Promise<SnapshotResponse>)
type ChannelSource = Channel[] | (() => Channel[])

function channel(id: number, name: string, maxConcurrency: number): Channel {
  return channelSchema.parse({
    id,
    type: 1,
    key: '',
    name,
    status: 1,
    max_concurrency: maxConcurrency,
    created_time: 1,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
  })
}

async function renderChannels(
  channels: ChannelSource,
  snapshotResponse: SnapshotSource = {
    data: { success: true, data: [] },
  }
) {
  const root = createRootRoute()
  const authenticated = createRoute({
    getParentRoute: () => root,
    id: '_authenticated',
  })
  const channelsRoute = createRoute({
    getParentRoute: () => authenticated,
    path: 'channels/',
    component: () => (
      <ChannelsProvider>
        <ChannelsTable />
      </ChannelsProvider>
    ),
  })
  const router = createRouter({
    routeTree: root.addChildren([authenticated.addChildren([channelsRoute])]),
    history: createMemoryHistory({ initialEntries: ['/channels/'] }),
  })
  const get = vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/channel') {
      const currentChannels =
        typeof channels === 'function' ? channels() : channels
      return {
        data: {
          success: true,
          data: {
            items: currentChannels,
            total: currentChannels.length,
            type_counts: {},
          },
        },
      }
    }
    if (url === '/api/channel/concurrency') {
      return typeof snapshotResponse === 'function'
        ? snapshotResponse()
        : snapshotResponse
    }
    return { data: { success: true, data: [] } }
  })
  await router.load()
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </I18nextProvider>
  )
  return get
}

function setMobileViewport() {
  const originalMatchMedia = window.matchMedia
  vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
    ...originalMatchMedia(query),
    matches: query === '(max-width: 640px)',
  }))
}

beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  localStorage.clear()
  useAuthStore.getState().auth.setUser({
    id: 2,
    username: 'reader',
    role: 10,
    permissions: {
      admin_permissions: { channel: { read: true, write: false } },
    },
  })
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
})

afterEach(() => {
  cleanup()
  client.clear()
  localStorage.clear()
  useAuthStore.getState().auth.setUser(originalUser)
  vi.restoreAllMocks()
})

test('updates every default desktop card with its live read-only snapshot', async () => {
  const channels = [
    channel(1, 'idle-load', 3),
    channel(2, 'busy-load', 3),
    channel(3, 'full-load', 3),
    channel(4, 'unlimited', 0),
  ]
  let resolveSnapshot!: (value: {
    data: { success: boolean; data: ChannelConcurrencyStatus[] }
  }) => void
  const snapshotResponse = new Promise<{
    data: { success: boolean; data: ChannelConcurrencyStatus[] }
  }>((resolve) => {
    resolveSnapshot = resolve
  })
  const get = await renderChannels(channels, snapshotResponse)

  const busyCard = (await screen.findByText('busy-load')).closest(
    '[data-slot="data-table-card"]'
  )
  expect(busyCard).not.toBeNull()
  expect(
    within(busyCard as HTMLElement).getByLabelText(
      'Channel concurrency unavailable'
    )
  ).toBeInTheDocument()

  await act(async () => {
    resolveSnapshot({
      data: {
        success: true,
        data: [
          {
            channel_id: 1,
            max_concurrency: 3,
            active: 0,
            waiting: 0,
            cooling_down: false,
          },
          {
            channel_id: 2,
            max_concurrency: 3,
            active: 2,
            waiting: 0,
            cooling_down: false,
          },
          {
            channel_id: 3,
            max_concurrency: 3,
            active: 3,
            waiting: 2,
            cooling_down: true,
          },
        ],
      },
    })
  })

  await waitFor(() =>
    expect(
      within(busyCard as HTMLElement).getByLabelText(
        'Channel concurrency: 2 of 3 active'
      )
    ).toBeInTheDocument()
  )
  const fullCard = screen
    .getByText('full-load')
    .closest('[data-slot="data-table-card"]') as HTMLElement
  expect(within(fullCard).getByText('+2')).toBeInTheDocument()
  expect(within(fullCard).getByText('Cooling')).toBeInTheDocument()
  const unlimitedCard = screen
    .getByText('unlimited')
    .closest('[data-slot="data-table-card"]') as HTMLElement
  expect(
    within(unlimitedCard).getByLabelText('Channel concurrency unavailable')
  ).toBeInTheDocument()
  expect(get).toHaveBeenCalledWith(
    '/api/channel/concurrency',
    expect.objectContaining({ params: { ids: '1,2,3' } })
  )
})

test('reuses the single-channel editor in the default desktop card', async () => {
  useAuthStore.getState().auth.setUser({
    id: 3,
    username: 'writer',
    role: 10,
    permissions: {
      admin_permissions: { channel: { read: true, write: true } },
    },
  })
  await renderChannels([channel(2, 'editable-card', 3)])

  const card = (await screen.findByText('editable-card')).closest(
    '[data-slot="data-table-card"]'
  ) as HTMLElement
  expect(
    within(card).getByRole('group', { name: 'Maximum Concurrency' })
  ).toBeVisible()
})

test('refreshes and switches the bounded snapshot set after persisted edits', async () => {
  useAuthStore.getState().auth.setUser({
    id: 3,
    username: 'writer',
    role: 10,
    permissions: {
      admin_permissions: { channel: { read: true, write: true } },
    },
  })
  let persisted = channel(2, 'changing-limit', 3)
  const get = await renderChannels(
    () => [persisted],
    () => ({
      data: {
        success: true,
        data:
          persisted.max_concurrency > 0
            ? [
                {
                  channel_id: persisted.id,
                  max_concurrency: persisted.max_concurrency,
                  active: 1,
                  waiting: 0,
                  cooling_down: false,
                },
              ]
            : [],
      },
    })
  )
  const put = vi.spyOn(api, 'put').mockImplementation(async (_url, body) => {
    const next = (body as { max_concurrency: number }).max_concurrency
    persisted = { ...persisted, max_concurrency: next }
    return { data: { success: true, data: persisted } }
  })
  const user = userEvent.setup()

  await screen.findByLabelText('Channel concurrency: 1 of 3 active')
  let editor = screen.getByRole('group', { name: 'Maximum Concurrency' })
  await user.click(within(editor).getByRole('button', { name: '3' }))
  await user.clear(within(editor).getByRole('textbox'))
  await user.type(within(editor).getByRole('textbox'), '5{Enter}')
  await screen.findByLabelText('Channel concurrency: 1 of 5 active')
  expect(put).toHaveBeenLastCalledWith(
    '/api/channel/',
    { id: 2, max_concurrency: 5 },
    expect.anything()
  )

  editor = screen.getByRole('group', { name: 'Maximum Concurrency' })
  await user.click(within(editor).getByRole('button', { name: '5' }))
  await user.clear(within(editor).getByRole('textbox'))
  await user.type(within(editor).getByRole('textbox'), '0{Enter}')
  await waitFor(() =>
    expect(
      screen.getByLabelText('Channel concurrency unavailable')
    ).toBeVisible()
  )
  expect(
    screen.queryByLabelText('Channel concurrency: 1 of 5 active')
  ).not.toBeInTheDocument()

  editor = screen.getByRole('group', { name: 'Maximum Concurrency' })
  await user.click(within(editor).getByRole('button', { name: '0' }))
  await user.clear(within(editor).getByRole('textbox'))
  await user.type(within(editor).getByRole('textbox'), '2{Enter}')
  await screen.findByLabelText('Channel concurrency: 1 of 2 active')

  const snapshotCalls = get.mock.calls.filter(
    ([url]) => url === '/api/channel/concurrency'
  )
  expect(snapshotCalls.length).toBeGreaterThanOrEqual(3)
  expect(snapshotCalls.at(-1)?.[1]).toEqual(
    expect.objectContaining({ params: { ids: '2' } })
  )
})

test('shows a default-visible non-sortable read-only concurrency column on desktop', async () => {
  localStorage.setItem('channels:view-mode', 'table')
  const channels = [channel(2, 'busy-table', 3), channel(4, 'unlimited', 0)]
  await renderChannels(channels, {
    data: {
      success: true,
      data: [
        {
          channel_id: 2,
          max_concurrency: 3,
          active: 2,
          waiting: 0,
          cooling_down: false,
        },
      ],
    },
  })

  expect(screen.getByRole('button', { name: 'Table view' })).toHaveAttribute(
    'aria-pressed',
    'true'
  )
  await screen.findByText('busy-table')
  const header = await waitFor(() => {
    const element = document.querySelector(
      'th[data-column-id="max_concurrency"]'
    )
    expect(element).not.toBeNull()
    return element as HTMLElement
  })
  expect(header).toHaveTextContent('Concurrency')
  expect(header).not.toHaveClass('hidden')
  expect(within(header).queryByRole('button')).not.toBeInTheDocument()
  const row = screen.getByText('busy-table').closest('tr') as HTMLElement
  const concurrencyCell = row.querySelector(
    '[data-column-id="max_concurrency"]'
  ) as HTMLElement
  expect(
    within(concurrencyCell).getByLabelText('Channel concurrency: 2 of 3 active')
  ).toBeInTheDocument()
  expect(
    within(concurrencyCell).queryByRole('spinbutton')
  ).not.toBeInTheDocument()
})

test('shows the single-channel editor in a desktop table row with ChannelWrite', async () => {
  useAuthStore.getState().auth.setUser({
    id: 3,
    username: 'writer',
    role: 10,
    permissions: {
      admin_permissions: { channel: { read: true, write: true } },
    },
  })
  localStorage.setItem('channels:view-mode', 'table')
  await renderChannels([channel(2, 'editable-table', 3)], {
    data: {
      success: true,
      data: [
        {
          channel_id: 2,
          max_concurrency: 3,
          active: 1,
          waiting: 0,
          cooling_down: false,
        },
      ],
    },
  })

  const row = (await screen.findByText('editable-table')).closest(
    'tr'
  ) as HTMLElement
  const concurrencyCell = row.querySelector(
    '[data-column-id="max_concurrency"]'
  ) as HTMLElement
  expect(
    within(concurrencyCell).getByRole('group', {
      name: 'Maximum Concurrency',
    })
  ).toBeVisible()
})

test('keeps a saved concurrency-column preference', async () => {
  localStorage.setItem('channels:view-mode', 'table')
  localStorage.setItem(
    'channels:column-visibility',
    JSON.stringify({ max_concurrency: false })
  )
  await renderChannels([channel(2, 'hidden-column', 3)])
  await screen.findByText('hidden-column')
  expect(screen.getByRole('button', { name: 'Table view' })).toHaveAttribute(
    'aria-pressed',
    'true'
  )

  expect(
    document.querySelector('[data-column-id="max_concurrency"]')
  ).toBeNull()
})

test('queries tag children but keeps the tag summary snapshot unavailable', async () => {
  localStorage.setItem('enable-tag-mode', 'true')
  const channels = [
    { ...channel(10, 'first-child', 2), tag: 'pool' },
    { ...channel(11, 'second-child', 4), tag: 'pool' },
  ]
  const get = await renderChannels(channels, {
    data: {
      success: true,
      data: [
        {
          channel_id: 10,
          max_concurrency: 2,
          active: 1,
          waiting: 0,
          cooling_down: false,
        },
        {
          channel_id: 11,
          max_concurrency: 4,
          active: 2,
          waiting: 0,
          cooling_down: false,
        },
      ],
    },
  })

  await waitFor(() =>
    expect(get).toHaveBeenCalledWith(
      '/api/channel',
      expect.objectContaining({
        params: expect.objectContaining({ tag_mode: true }),
      })
    )
  )

  const renderedName = await screen.findByText('Tag：pool')
  const tagCard = renderedName.closest(
    '[data-slot="data-table-card"]'
  ) as HTMLElement
  expect(
    within(tagCard).getByLabelText('Channel concurrency unavailable')
  ).toBeInTheDocument()
  await waitFor(() =>
    expect(get).toHaveBeenCalledWith(
      '/api/channel/concurrency',
      expect.objectContaining({ params: { ids: '10,11' } })
    )
  )
})

test('hides concurrency from both mobile cards and the mobile table', async () => {
  setMobileViewport()
  const channels = [channel(2, 'mobile-channel', 3)]
  await renderChannels(channels, {
    data: {
      success: true,
      data: [
        {
          channel_id: 2,
          max_concurrency: 3,
          active: 2,
          waiting: 1,
          cooling_down: true,
        },
      ],
    },
  })

  const card = (await screen.findByText('mobile-channel')).closest(
    '[data-slot="data-table-card"]'
  ) as HTMLElement
  expect(within(card).queryByText('Concurrency')).not.toBeInTheDocument()
  expect(
    within(card).queryByLabelText(/Channel concurrency/)
  ).not.toBeInTheDocument()

  await userEvent.click(screen.getByRole('button', { name: 'Table view' }))
  await waitFor(() => {
    const mobileColumn = document.querySelectorAll(
      '[data-column-id="max_concurrency"]'
    )
    expect(mobileColumn.length).toBeGreaterThan(1)
    for (const element of mobileColumn) {
      expect(element).toHaveClass('hidden')
    }
  })
})
