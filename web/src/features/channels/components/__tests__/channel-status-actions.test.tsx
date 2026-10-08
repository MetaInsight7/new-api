import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { channelsQueryKeys } from '../../lib'
import { channelSchema, type Channel } from '../../types'
import { ChannelsProvider } from '../channels-provider'
import { DataTableRowActions } from '../data-table-row-actions'

const clients: QueryClient[] = []
const previousUser = useAuthStore.getState().auth.user

function StatusActions(props: { channel: Channel }) {
  const table = useReactTable({
    data: [props.channel],
    columns: [{ accessorKey: 'id' }],
    getCoreRowModel: getCoreRowModel(),
  })
  return <DataTableRowActions row={table.getRowModel().rows[0]} />
}

function renderStatusActions(status: number, type = 1) {
  const channel = channelSchema.parse({
    id: 7,
    type,
    key: '',
    name: 'Upstream channel',
    status,
    created_time: 0,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  client.setQueryData(channelsQueryKeys.lists(), [channel])
  clients.push(client)
  render(
    <QueryClientProvider client={client}>
      <ChannelsProvider>
        <StatusActions channel={channel} />
      </ChannelsProvider>
    </QueryClientProvider>
  )
  return client
}

beforeEach(() => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'root', role: 100 })
})

afterEach(() => {
  clients.splice(0).forEach((client) => client.clear())
  useAuthStore.getState().auth.setUser(previousUser)
  vi.restoreAllMocks()
})

it('disables permanent status actions without channel operate permission', async () => {
  useAuthStore.getState().auth.setUser({
    id: 2,
    username: 'reader',
    role: 10,
    permissions: {
      admin_permissions: { channel: { read: true, operate: false } },
    },
  })
  const post = vi.spyOn(api, 'post')
  renderStatusActions(1)
  const user = userEvent.setup()
  expect(screen.getByRole('button', { name: 'Disable' })).toBeDisabled()
  await user.click(screen.getByRole('button', { name: 'Open menu' }))
  const action = await screen.findByRole('menuitem', { name: 'Mark as Banned' })
  expect(action).toHaveAttribute('aria-disabled', 'true')
  await user.click(action)
  expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  expect(post).not.toHaveBeenCalled()
})

it('keeps confirmation open and leaves the channel cache unchanged after a failed status request', async () => {
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValueOnce({
      data: { success: false, message: 'Status update rejected' },
    })
    .mockResolvedValueOnce({ data: { success: true, data: true } })
  const client = renderStatusActions(1)
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: 'Open menu' }))
  await user.click(
    await screen.findByRole('menuitem', { name: 'Mark as Banned' })
  )
  await user.click(
    await screen.findByRole('button', { name: 'Mark as Banned' })
  )
  await waitFor(() => expect(post).toHaveBeenCalledTimes(1))
  expect(screen.getByRole('alertdialog')).toBeVisible()
  expect(client.getQueryState(channelsQueryKeys.lists())?.isInvalidated).toBe(
    false
  )
  await user.click(screen.getByRole('button', { name: 'Mark as Banned' }))
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
  expect(client.getQueryState(channelsQueryKeys.lists())?.isInvalidated).toBe(
    true
  )
})

it('disables confirmation and cancel while a permanent status request is pending', async () => {
  let release!: () => void
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  const post = vi.spyOn(api, 'post').mockImplementation(async () => {
    await gate
    return { data: { success: true, data: true } }
  })
  renderStatusActions(1)
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: 'Open menu' }))
  await user.click(
    await screen.findByRole('menuitem', { name: 'Mark as Banned' })
  )
  await user.click(
    await screen.findByRole('button', { name: 'Mark as Banned' })
  )
  expect(screen.getByRole('button', { name: 'Mark as Banned' })).toBeDisabled()
  expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled()
  expect(post).toHaveBeenCalledTimes(1)
  await act(async () => {
    release()
    await gate
  })
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
})

it.each([
  { status: 1, type: 1, action: 'Mark as Banned', target: 4 },
  { status: 4, type: 57, action: 'Unban', target: 1 },
])(
  'confirms $action for channel type $type before updating its status',
  async (testCase) => {
    const post = vi.spyOn(api, 'post').mockResolvedValue({
      data: { success: true, data: true },
    })
    const client = renderStatusActions(testCase.status, testCase.type)
    const user = userEvent.setup()

    if (testCase.status === 4) {
      expect(screen.getByRole('button', { name: 'Enable' })).toBeDisabled()
    }
    await user.click(screen.getByRole('button', { name: 'Open menu' }))
    await user.click(
      await screen.findByRole('menuitem', { name: testCase.action })
    )
    expect(await screen.findByRole('alertdialog')).toBeVisible()
    expect(post).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: testCase.action }))
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
    expect(post.mock.calls[0]?.slice(0, 2)).toEqual([
      '/api/channel/7/status',
      { status: testCase.target },
    ])
    expect(client.getQueryState(channelsQueryKeys.lists())?.isInvalidated).toBe(
      true
    )
  }
)
