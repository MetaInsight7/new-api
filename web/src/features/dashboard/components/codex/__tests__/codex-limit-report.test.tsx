import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  act,
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { toIntlLocale } from '@/i18n/languages'
import { api } from '@/lib/api'
import { formatNumber, formatQuota } from '@/lib/format'
import { useSystemConfigStore } from '@/stores/system-config-store'

import type { CodexLimitReport, CodexLimitReportRow } from '../../../types'
import { CodexChannelReport } from '../codex-channel-report'
import { CodexLimitReportPanel } from '../codex-limit-report'
import { CodexWindowMeters } from '../codex-window-meters'

const freeRow: CodexLimitReportRow = {
  channel_id: 1,
  channel_name: 'Free account',
  channel_status: 2,
  success: true,
  allowed: false,
  limit_reached: true,
  plan_type: 'free',
  email: 'free@example.test',
  range_token_used: 1234,
  range_quota: 500000,
  base_weekly_window: {
    used_percent: 23,
    reset_at: 1791368100,
    limit_window_seconds: 604740,
  },
  additional_limits: [
    {
      name: 'Images',
      metered_feature: 'image-generation',
      unknown_windows: [{ used_percent: 0 }],
    },
  ],
}
const failedRow: CodexLimitReportRow = {
  channel_id: 2,
  channel_name: 'Failed account',
  channel_status: 4,
  success: false,
  upstream_status: 503,
  message: 'Failed to fetch Codex usage',
  range_token_used: 80,
  range_quota: 50000,
}

function reportFixture(
  overrides: Partial<CodexLimitReport> = {}
): CodexLimitReport {
  return {
    generated_at: 1791368100,
    start_timestamp: 1790763300,
    end_timestamp: 1791368100,
    total_channels: 2,
    success_count: 1,
    failure_count: 1,
    total_token_used: 1314,
    total_quota: 550000,
    rows: [freeRow, failedRow],
    ...overrides,
  }
}

let queryClient: QueryClient

beforeEach(async () => {
  window.localStorage.clear()
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
  await i18next.changeLanguage('en')
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
})

afterEach(async () => {
  cleanup()
  queryClient.clear()
  vi.useRealTimers()
  await i18next.changeLanguage('en')
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
})

function renderReport(): void {
  render(
    <QueryClientProvider client={queryClient}>
      <CodexLimitReportPanel />
    </QueryClientProvider>
  )
}

describe('Codex limits dashboard', () => {
  test('shows query counts without equating a successful limited account with availability', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: reportFixture() },
    })
    renderReport()
    expect(
      await screen.findByRole('tab', { name: 'Free account' })
    ).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByText('Limited')).toBeVisible()
    expect(screen.queryByText('Available')).not.toBeInTheDocument()
    expect(screen.getAllByText('Query succeeded')).toHaveLength(2)
    expect(screen.getByText(formatQuota(550000))).toBeVisible()
    expect(screen.getByText('Range Tokens').parentElement).toHaveTextContent(
      '1,234'
    )
    expect(
      screen.getByText(
        'Upstream limits are a current snapshot; local usage follows the selected time range.'
      )
    ).toBeVisible()
    expect(screen.getByRole('heading', { name: 'Weekly Window' })).toBeVisible()
    expect(
      screen.queryByRole('heading', { name: '5-Hour Window' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Images' })).toBeVisible()
    expect(
      screen.getByRole('progressbar', { name: 'Rate Limit Windows: 0%' })
    ).toHaveAttribute('aria-valuenow', '0')
  })

  test('keeps failed channels and their local usage accessible through keyboard tabs', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: reportFixture() },
    })
    renderReport()
    const firstTab = await screen.findByRole('tab', { name: 'Free account' })
    firstTab.focus()
    const user = userEvent.setup()
    await user.keyboard('{ArrowRight}{Enter}')
    expect(screen.getByRole('tab', { name: /Failed account/ })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    const panel = screen.getByRole('tabpanel')
    expect(within(panel).getByText('Failed to fetch Codex usage')).toBeVisible()
    expect(within(panel).getByText('503')).toBeVisible()
    expect(
      within(panel).getByText('Range Tokens').parentElement
    ).toHaveTextContent('80')
    expect(within(panel).queryByRole('progressbar')).not.toBeInTheDocument()
  })

  test('uses rolling 7 days by default and updates timestamps on range changes and refresh', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-10-07T10:15:00Z'))
    const request = vi
      .spyOn(api, 'get')
      .mockResolvedValue({ data: { success: true, data: reportFixture() } })
    renderReport()
    await screen.findByRole('tab', { name: 'Free account' })
    expect(request).toHaveBeenLastCalledWith(
      '/api/data/codex/limits',
      expect.objectContaining({
        params: { start_timestamp: 1790763300, end_timestamp: 1791368100 },
        signal: expect.any(AbortSignal),
        disableDuplicate: true,
      })
    )
    expect(screen.getByRole('button', { name: '7 Days' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: '1 Day' }))
    await waitFor(() =>
      expect(request).toHaveBeenLastCalledWith(
        '/api/data/codex/limits',
        expect.objectContaining({
          params: { start_timestamp: 1791281700, end_timestamp: 1791368100 },
        })
      )
    )
    expect(screen.getByRole('button', { name: '1 Day' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    vi.setSystemTime(new Date('2026-10-07T10:16:00Z'))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Refresh' })).toBeEnabled()
    )
    await user.click(screen.getByRole('button', { name: 'Refresh' }))
    await waitFor(() =>
      expect(request).toHaveBeenLastCalledWith(
        '/api/data/codex/limits',
        expect.objectContaining({
          params: { start_timestamp: 1791281760, end_timestamp: 1791368160 },
        })
      )
    )
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Refresh' })).toBeEnabled()
    )
    await user.click(screen.getByRole('button', { name: 'Refresh' }))
    await waitFor(() => expect(request).toHaveBeenCalledTimes(4))
  })

  test('disables refresh while loading and cancels obsolete interval requests', async () => {
    const request = vi
      .spyOn(api, 'get')
      .mockImplementationOnce(() => new Promise(() => {}))
      .mockResolvedValue({
        data: { success: true, data: reportFixture() },
      })
    renderReport()
    expect(screen.getByText('Loading Codex limits...')).toBeVisible()
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeDisabled()
    const signal = request.mock.calls[0][1]?.signal
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: '14 Days' }))
    await screen.findByRole('tab', { name: 'Free account' })
    expect(signal?.aborted).toBe(true)
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeEnabled()
  })

  test.each([
    { success: false, message: 'Report permission denied' },
    { success: true, data: undefined },
  ])(
    'shows business or malformed failures and retries explicitly',
    async (response) => {
      vi.spyOn(api, 'get')
        .mockResolvedValueOnce({ data: response })
        .mockResolvedValue({ data: { success: true, data: reportFixture() } })
      renderReport()
      const retry = await screen.findByRole('button', { name: 'Retry' })
      expect(screen.queryByRole('tab')).not.toBeInTheDocument()
      if (!response.success) {
        expect(screen.getByText('Report permission denied')).toBeVisible()
      }
      const user = userEvent.setup()
      await user.click(retry)
      expect(
        await screen.findByRole('tab', { name: 'Free account' })
      ).toBeVisible()
    }
  )

  test('shows a recoverable transport failure without losing the retry control', async () => {
    vi.spyOn(api, 'get').mockRejectedValue(new Error('Network unavailable'))
    renderReport()
    expect(await screen.findByRole('button', { name: 'Retry' })).toBeEnabled()
    expect(screen.getByText('Network unavailable')).toBeVisible()
  })

  test('shows an explicit empty state when no Codex channels exist', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: {
        success: true,
        data: reportFixture({
          total_channels: 0,
          success_count: 0,
          failure_count: 0,
          total_token_used: 0,
          total_quota: 0,
          rows: [],
        }),
      },
    })
    renderReport()
    expect(await screen.findByText('No Codex channels found')).toBeVisible()
    expect(screen.queryByRole('tab')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeEnabled()
  })

  test('handles long account identities and falls back when the selected channel disappears', async () => {
    const identity = `${'long-identity-'.repeat(30)}@example.test`
    const longRow = {
      ...freeRow,
      channel_name: identity,
      email: identity,
      account_id: identity,
    }
    const view = render(<CodexChannelReport rows={[failedRow, longRow]} />)
    const user = userEvent.setup()
    await user.click(screen.getByRole('tab', { name: identity }))
    expect(screen.getByRole('tab', { name: identity })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(
      within(screen.getByRole('tabpanel')).getAllByText(identity)
    ).toHaveLength(3)
    view.rerender(<CodexChannelReport rows={[failedRow]} />)
    expect(screen.getByRole('tab', { name: /Failed account/ })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(screen.getByText('Failed to fetch Codex usage')).toBeVisible()
  })

  test('updates number formatting when switching between Simplified and Traditional Chinese', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: reportFixture() },
    })
    renderReport()
    await screen.findByRole('tab', { name: 'Free account' })
    for (const language of ['zhCN', 'zhTW']) {
      await act(() => i18next.changeLanguage(language))
      expect(screen.getByText('Range Tokens').parentElement).toHaveTextContent(
        formatNumber(1234, toIntlLocale(language))
      )
      expect(
        screen.getByRole('progressbar', { name: 'Weekly Window: 23%' })
      ).toBeVisible()
    }
  })
})

describe('Codex limit meters', () => {
  test.each([0, 10])(
    'retains a real %i-second reset without missing-window placeholders or fabricated usage',
    (seconds) => {
      render(
        <CodexWindowMeters
          unknownWindows={[
            { reset_after_seconds: seconds, limit_window_seconds: 43200 },
          ]}
        />
      )
      expect(
        screen.getByRole('heading', { name: 'Rate Limit Windows' })
      ).toBeVisible()
      expect(
        screen.queryByRole('heading', { name: '5-Hour Window' })
      ).not.toBeInTheDocument()
      expect(
        screen.queryByRole('heading', { name: 'Weekly Window' })
      ).not.toBeInTheDocument()
      expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
      expect(screen.getByText(`0m ${seconds}s`)).toBeVisible()
    }
  )

  test('uses an explicit no-windows message rather than absent weekly and hourly meters', () => {
    render(<CodexWindowMeters />)
    expect(screen.getByText('No rate-limit windows returned')).toBeVisible()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })
})
