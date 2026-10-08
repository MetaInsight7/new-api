import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { CodexUsageDialog } from '../dialogs/codex-usage-dialog'

const response = {
  success: true,
  upstream_status: 200,
  data: {
    account_id: 'account-test',
    plan_type: 'plus',
    rate_limit: {
      primary_window: { used_percent: 20, limit_window_seconds: 5 * 3600 },
    },
    rate_limit_reset_credits: { available_count: 1 },
  },
}

const previousUser = useAuthStore.getState().auth.user

beforeEach(() => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'root', role: 100 })
})

afterEach(() => {
  useAuthStore.getState().auth.setUser(previousUser)
})

describe('Codex account windows and safe reset feedback', () => {
  test('lets read-only admins inspect credits without offering a reset action', async () => {
    useAuthStore.getState().auth.setUser({
      id: 2,
      username: 'viewer',
      role: 10,
      permissions: { admin_permissions: { channel: { read: true } } },
    })
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: { available_count: 1, credits: [] } },
    })
    const post = vi.spyOn(api, 'post')
    render(
      <CodexUsageDialog
        open
        onOpenChange={vi.fn()}
        channelId={9}
        response={response}
      />
    )
    await userEvent.click(screen.getByRole('button', { name: /Reset Credits/ }))
    expect(await screen.findByText('Available reset credits')).toBeVisible()
    expect(
      screen.queryByRole('button', { name: 'Apply reset' })
    ).not.toBeInTheDocument()
    expect(post).not.toHaveBeenCalled()
  })

  test('shows account ID and real windows without fabricated missing quota', () => {
    render(
      <CodexUsageDialog
        open
        onOpenChange={vi.fn()}
        channelId={9}
        response={{
          ...response,
          data: {
            ...response.data,
            rate_limit: {
              primary_window: {
                used_percent: 20,
                limit_window_seconds: 42 * 3600,
              },
            },
            additional_rate_limits: [
              {
                limit_name: 'model-extra',
                primary_window: {
                  used_percent: 10,
                  limit_window_seconds: 604800,
                },
              },
            ],
          },
        }}
      />
    )
    expect(screen.getByText('account-test')).toBeVisible()
    expect(screen.queryByText('5-Hour Window')).not.toBeInTheDocument()
    expect(screen.getByText('Weekly Window')).toBeVisible()
    expect(screen.getByText(/Quota window/)).toBeVisible()
    expect(screen.getByText('model-extra')).toBeVisible()
  })

  test('distinguishes successful consumption from a failed usage refresh', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: { available_count: 1, credits: [] } },
    })
    const post = vi.spyOn(api, 'post').mockResolvedValue({
      data: { success: true, data: { windows_reset: 1 } },
    })
    render(
      <CodexUsageDialog
        open
        onOpenChange={vi.fn()}
        channelId={9}
        response={response}
        onRefresh={() => Promise.resolve(false)}
      />
    )
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /Reset Credits/ }))
    await user.click(await screen.findByRole('button', { name: 'Apply reset' }))
    const confirmation = await screen.findByRole('alertdialog', {
      name: 'Confirm usage reset',
    })
    await user.click(
      within(confirmation).getByRole('button', { name: 'Apply reset' })
    )
    expect(await screen.findByText('Reset completed')).toBeVisible()
    expect(
      await screen.findByText(/Reset succeeded, but refreshing details failed/)
    ).toBeVisible()
    expect(screen.queryByText('Reset failed')).not.toBeInTheDocument()
    expect(
      screen.queryByText('Reset completed. Latest usage has been refreshed.')
    ).not.toBeInTheDocument()
    expect(post).toHaveBeenCalledOnce()
  })

  test('does not claim zero reset windows as a successful reset', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: { available_count: 1, credits: [] } },
    })
    vi.spyOn(api, 'post').mockResolvedValue({
      data: { success: true, data: { windows_reset: 0 } },
    })
    render(
      <CodexUsageDialog
        open
        onOpenChange={vi.fn()}
        channelId={9}
        response={response}
        onRefresh={() => Promise.resolve(true)}
      />
    )
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /Reset Credits/ }))
    await user.click(await screen.findByRole('button', { name: 'Apply reset' }))
    const confirmation = await screen.findByRole('alertdialog', {
      name: 'Confirm usage reset',
    })
    await user.click(
      within(confirmation).getByRole('button', { name: 'Apply reset' })
    )
    await user.click(
      within(confirmation).getByRole('button', { name: 'Cancel' })
    )
    expect(
      await screen.findByText('Upstream did not confirm any reset windows.')
    ).toBeVisible()
    await waitFor(() =>
      expect(screen.queryByText('Reset completed')).not.toBeInTheDocument()
    )
  })
})
