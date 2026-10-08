import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { CodexModelGovernance } from '..'
import {
  getCodexModelGovernanceRecords,
  reviewCodexModelGovernanceRecord,
  testCodexModelGovernanceRule,
  triggerCodexModelGovernanceTask,
} from '../api'
import { GovernanceReviewActions } from '../review-actions'
import type { CodexModelGovernanceRecord } from '../types'

const record: CodexModelGovernanceRecord = {
  id: 7,
  model_name: 'configured-model',
  status: 'removed',
  source: 'probe',
  matched_rule: 'strict rule',
  last_error: 'unsupported model',
  affected_channel_ids: [11, 12],
  disabled_channel_ids: [11, 12],
  abilities_disabled: true,
  detected_at: 0,
  last_checked_at: 0,
  last_alerted_at: 0,
  reviewed_at: 0,
  reviewed_by: 1,
  review_note: '',
}
const previousUser = useAuthStore.getState().auth.user
let client: QueryClient

beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  useAuthStore.getState().auth.setUser({ id: 1, username: 'root', role: 100 })
})

afterEach(() => {
  client.clear()
  useAuthStore.getState().auth.setUser(previousUser)
})

it('rejects business-envelope failures for reads, reviews, rule tests, and triggers', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: false, message: 'Read denied' },
  })
  vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: false, message: 'Write denied' },
  })
  await expect(getCodexModelGovernanceRecords()).rejects.toThrow('Read denied')
  await expect(
    reviewCodexModelGovernanceRecord(7, 'restore', '')
  ).rejects.toThrow('Write denied')
  await expect(
    testCodexModelGovernanceRule('message', ['pattern'])
  ).rejects.toThrow('Write denied')
  await expect(triggerCodexModelGovernanceTask('probe')).rejects.toThrow(
    'Write denied'
  )
})

it('keeps a failed restore open without discarding the review decision', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: [{ ...record, status: 'unsupported_disabled' }],
    },
  })
  const reviewRequest = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: false, message: 'Review denied' } })
  render(
    <QueryClientProvider client={client}>
      <CodexModelGovernance />
    </QueryClientProvider>
  )
  await userEvent.click(
    await screen.findByRole('button', { name: 'Restore model' })
  )
  expect(screen.getByText('Save this review decision?')).toBeVisible()
  await userEvent.click(screen.getByRole('button', { name: 'Confirm' }))
  await waitFor(() =>
    expect(reviewRequest).toHaveBeenCalledWith(
      '/api/codex_model_governance/7/review/restore',
      { action: 'restore', note: '' }
    )
  )
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Confirm' })).toBeEnabled()
  )
  expect(screen.getByRole('alertdialog')).toBeVisible()
})

it('does not offer restore or other mutations for removed channel configuration', () => {
  render(
    <GovernanceReviewActions
      record={record}
      pending={false}
      onReview={vi.fn()}
    />
  )
  expect(screen.queryAllByRole('button')).toHaveLength(0)
})

it('shows the refreshed persisted review note after a successful decision', async () => {
  vi.spyOn(api, 'get')
    .mockResolvedValue({
      data: {
        success: true,
        data: [
          { ...record, status: 'active', review_note: 'saved review note' },
        ],
      },
    })
    .mockResolvedValueOnce({
      data: {
        success: true,
        data: [{ ...record, status: 'unsupported_disabled' }],
      },
    })
  const reviewRequest = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: true } })
  render(
    <QueryClientProvider client={client}>
      <CodexModelGovernance />
    </QueryClientProvider>
  )
  await userEvent.type(
    await screen.findByRole('textbox', { name: 'Review note' }),
    'original draft'
  )
  await userEvent.click(screen.getByRole('button', { name: 'Restore model' }))
  await userEvent.click(screen.getByRole('button', { name: 'Confirm' }))
  await waitFor(() =>
    expect(reviewRequest).toHaveBeenCalledWith(
      '/api/codex_model_governance/7/review/restore',
      { action: 'restore', note: 'original draft' }
    )
  )
  await waitFor(() =>
    expect(screen.getByRole('textbox', { name: 'Review note' })).toHaveValue(
      'saved review note'
    )
  )
})

it('offers scoped disable and restore but no ignore action when abilities are blocked', () => {
  render(
    <GovernanceReviewActions
      record={{
        ...record,
        status: 'unsupported_pending_review',
        disabled_channel_ids: [11],
      }}
      pending={false}
      onReview={vi.fn()}
    />
  )
  expect(screen.getByRole('button', { name: 'Disable model' })).toBeEnabled()
  expect(screen.getByRole('button', { name: 'Restore model' })).toBeEnabled()
  expect(
    screen.queryByRole('button', { name: 'Ignore finding' })
  ).not.toBeInTheDocument()
})

it('hides mutation controls for read-only admins and disables editing while pending', () => {
  useAuthStore.getState().auth.setUser({
    id: 2,
    username: 'viewer',
    role: 10,
    permissions: { admin_permissions: { channel: { read: true } } },
  })
  render(
    <GovernanceReviewActions
      record={record}
      pending={false}
      onReview={vi.fn()}
    />
  )
  expect(screen.queryAllByRole('button')).toHaveLength(0)
  expect(screen.getByRole('textbox', { name: 'Review note' })).toBeDisabled()
})

it('shows reusable error and empty states instead of an empty page', async () => {
  vi.spyOn(api, 'get')
    .mockResolvedValueOnce({ data: { success: false, message: 'Read denied' } })
    .mockResolvedValue({ data: { success: true, data: [] } })
  render(
    <QueryClientProvider client={client}>
      <CodexModelGovernance />
    </QueryClientProvider>
  )
  expect(
    await screen.findByText('Failed to load governance records')
  ).toBeVisible()
  await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
  expect(await screen.findByText('No Data')).toBeVisible()
})
