import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
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
import { channelsQueryKeys } from '../../lib'
import { channelSchema } from '../../types'
import { ChannelConcurrencyCell } from '../channel-concurrency-status'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  initAsync: false,
})

const originalUser = useAuthStore.getState().auth.user
let client: QueryClient

const channel = channelSchema.parse({
  id: 42,
  type: 1,
  key: '',
  name: 'bounded',
  status: 1,
  max_concurrency: 3,
  created_time: 1,
  test_time: 0,
  response_time: 0,
  balance_updated_time: 0,
})

const status: ChannelConcurrencyStatus = {
  channel_id: 42,
  max_concurrency: 3,
  active: 1,
  waiting: 0,
  cooling_down: false,
}

function renderConcurrencyCell() {
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <ChannelConcurrencyCell channel={channel} status={status} />
      </QueryClientProvider>
    </I18nextProvider>
  )
}

function grantChannelWrite() {
  useAuthStore.getState().auth.setUser({
    id: 3,
    username: 'writer',
    role: 10,
    permissions: {
      admin_permissions: { channel: { read: true, write: true } },
    },
  })
}

beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  useAuthStore.getState().auth.setUser({
    id: 2,
    username: 'reader',
    role: 10,
    permissions: {
      admin_permissions: { channel: { read: true, write: false } },
    },
  })
})

afterEach(() => {
  cleanup()
  client.clear()
  useAuthStore.getState().auth.setUser(originalUser)
  vi.restoreAllMocks()
})

test('keeps the live status read-only without ChannelWrite permission', () => {
  renderConcurrencyCell()

  expect(
    screen.getByLabelText('Channel concurrency: 1 of 3 active')
  ).toBeVisible()
  expect(
    screen.queryByRole('group', { name: 'Maximum Concurrency' })
  ).not.toBeInTheDocument()
})

test('shows the single-channel editor with ChannelWrite permission', () => {
  grantChannelWrite()

  renderConcurrencyCell()

  expect(
    screen.getByRole('group', { name: 'Maximum Concurrency' })
  ).toBeVisible()
  expect(
    screen.getByLabelText('Channel concurrency: 1 of 3 active')
  ).toBeVisible()
})

test('persists a positive maximum concurrency from the inline editor', async () => {
  grantChannelWrite()
  client.setQueryData(channelsQueryKeys.detail(channel.id), channel)
  const put = vi.spyOn(api, 'put').mockResolvedValue({
    data: { success: true, data: { ...channel, max_concurrency: 7 } },
  })
  const user = userEvent.setup()
  renderConcurrencyCell()

  const editor = screen.getByRole('group', { name: 'Maximum Concurrency' })
  await user.click(within(editor).getByRole('button', { name: '3' }))
  const input = within(editor).getByRole('textbox')
  await user.clear(input)
  await user.type(input, '7{Enter}')

  await waitFor(() =>
    expect(put).toHaveBeenCalledWith(
      '/api/channel/',
      { id: 42, max_concurrency: 7 },
      expect.anything()
    )
  )
  await waitFor(() =>
    expect(
      client.getQueryState(channelsQueryKeys.detail(channel.id))?.isInvalidated
    ).toBe(true)
  )
})

test('persists an explicit zero to clear maximum concurrency', async () => {
  grantChannelWrite()
  const put = vi.spyOn(api, 'put').mockResolvedValue({
    data: { success: true, data: { ...channel, max_concurrency: 0 } },
  })
  const user = userEvent.setup()
  renderConcurrencyCell()

  const editor = screen.getByRole('group', { name: 'Maximum Concurrency' })
  await user.click(within(editor).getByRole('button', { name: '3' }))
  const input = within(editor).getByRole('textbox')
  await user.clear(input)
  await user.type(input, '0{Enter}')

  await waitFor(() =>
    expect(put).toHaveBeenCalledWith(
      '/api/channel/',
      { id: 42, max_concurrency: 0 },
      expect.anything()
    )
  )
})

test.each(['-1', '1.5'])(
  'rejects invalid inline value %s without sending it',
  async (invalidValue) => {
    grantChannelWrite()
    const put = vi.spyOn(api, 'put')
    const user = userEvent.setup()
    renderConcurrencyCell()

    const editor = screen.getByRole('group', { name: 'Maximum Concurrency' })
    await user.click(within(editor).getByRole('button', { name: '3' }))
    const input = within(editor).getByRole('textbox')
    await user.clear(input)
    await user.type(input, `${invalidValue}{Enter}`)

    expect(put).not.toHaveBeenCalled()
    expect(within(editor).getByRole('button', { name: '3' })).toBeVisible()
  }
)

test('restores the persisted value after failure and allows retry', async () => {
  grantChannelWrite()
  client.setQueryData(channelsQueryKeys.lists(), [channel])
  client.setQueryData(channelsQueryKeys.concurrency([42]), [status])
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValueOnce({
      data: { success: false, message: 'write rejected' },
    })
    .mockResolvedValueOnce({
      data: { success: true, data: { ...channel, max_concurrency: 5 } },
    })
  const user = userEvent.setup()
  renderConcurrencyCell()

  const editor = screen.getByRole('group', { name: 'Maximum Concurrency' })
  await user.click(within(editor).getByRole('button', { name: '3' }))
  await user.clear(within(editor).getByRole('textbox'))
  await user.type(within(editor).getByRole('textbox'), '5{Enter}')

  await waitFor(() => expect(put).toHaveBeenCalledTimes(1))
  expect(within(editor).getByRole('button', { name: '3' })).toBeVisible()
  expect(
    screen.getByLabelText('Channel concurrency: 1 of 3 active')
  ).toBeVisible()
  expect(client.getQueryState(channelsQueryKeys.lists())?.isInvalidated).toBe(
    false
  )
  expect(
    client.getQueryState(channelsQueryKeys.concurrency([42]))?.isInvalidated
  ).toBe(false)

  await user.click(within(editor).getByRole('button', { name: '3' }))
  await user.clear(within(editor).getByRole('textbox'))
  await user.type(within(editor).getByRole('textbox'), '5{Enter}')

  await waitFor(() => expect(put).toHaveBeenCalledTimes(2))
  await waitFor(() =>
    expect(client.getQueryState(channelsQueryKeys.lists())?.isInvalidated).toBe(
      true
    )
  )
  expect(
    client.getQueryState(channelsQueryKeys.concurrency([42]))?.isInvalidated
  ).toBe(true)
})

test('keeps a successful saved value when cache refresh fails', async () => {
  grantChannelWrite()
  vi.spyOn(api, 'put').mockResolvedValue({
    data: { success: true, data: { ...channel, max_concurrency: 7 } },
  })
  vi.spyOn(client, 'invalidateQueries').mockRejectedValueOnce(
    new Error('refresh failed')
  )
  const user = userEvent.setup()
  renderConcurrencyCell()

  const editor = screen.getByRole('group', { name: 'Maximum Concurrency' })
  await user.click(within(editor).getByRole('button', { name: '3' }))
  await user.clear(within(editor).getByRole('textbox'))
  await user.type(within(editor).getByRole('textbox'), '7{Enter}')

  await waitFor(() =>
    expect(within(editor).getByRole('button', { name: '7' })).toBeVisible()
  )
})

test('never exposes an editor for a tag aggregate row', () => {
  grantChannelWrite()
  const tagRow = {
    ...channel,
    id: 0,
    name: 'tag aggregate',
    children: [channel],
  }

  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <ChannelConcurrencyCell channel={tagRow} />
      </QueryClientProvider>
    </I18nextProvider>
  )

  expect(
    screen.queryByRole('group', { name: 'Maximum Concurrency' })
  ).not.toBeInTheDocument()
  expect(
    screen.getByLabelText('Channel concurrency unavailable')
  ).toHaveTextContent('-')
})
