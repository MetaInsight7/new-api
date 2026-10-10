import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { Route as OperationsRoute } from '@/routes/_authenticated/system-settings/operations/$section'

import { OperationsSettings } from '..'
import { SettingsPageProvider } from '../../components/settings-page-context'
import {
  ChannelConcurrencySettingsSection,
  type ChannelConcurrencySettingsValues,
} from '../channel-concurrency-settings-section'

const defaults: ChannelConcurrencySettingsValues = {
  'channel_concurrency_setting.wait_enabled': true,
  'channel_concurrency_setting.wait_timeout_ms': 5000,
  'channel_concurrency_setting.max_waiting_per_channel': 0,
  'channel_concurrency_setting.cooldown_enabled': true,
  'channel_concurrency_setting.cooldown_seconds': 30,
  'channel_concurrency_setting.cooldown_on_status_429': true,
  'channel_concurrency_setting.cooldown_on_message_match': false,
  'channel_concurrency_setting.load_cache_enabled': true,
  'channel_concurrency_setting.slot_ttl_minutes': 30,
  'channel_concurrency_setting.wait_interval_ms': 100,
  'channel_concurrency_setting.load_cache_ttl_ms': 200,
  'channel_concurrency_setting.max_acquire_attempts': 8,
}

function Fixture({
  defaultValues = defaults,
}: {
  defaultValues?: ChannelConcurrencySettingsValues
}) {
  const [actionsContainer, setActionsContainer] =
    useState<HTMLDivElement | null>(null)

  return (
    <>
      <div ref={setActionsContainer} />
      <SettingsPageProvider
        actionsContainer={actionsContainer}
        suppressSectionHeader={false}
      >
        <ChannelConcurrencySettingsSection defaultValues={defaultValues} />
      </SettingsPageProvider>
    </>
  )
}

describe('channel concurrency operations settings', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })
    vi.spyOn(api, 'put').mockResolvedValue({ data: { success: true } })
  })

  afterEach(() => queryClient.clear())

  it('shows all common and advanced policies with the confirmed defaults', async () => {
    render(
      <QueryClientProvider client={queryClient}>
        <Fixture />
      </QueryClientProvider>
    )

    expect(
      screen.getByRole('heading', { name: 'Channel Concurrency Limits' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('heading', { name: 'Common policies' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('heading', { name: 'Advanced runtime parameters' })
    ).toBeInTheDocument()

    expect(
      screen.getByRole('switch', {
        name: 'Wait when channel concurrency is full',
      })
    ).toBeChecked()
    expect(
      screen.getByRole('spinbutton', { name: 'Wait timeout (milliseconds)' })
    ).toHaveValue(5000)
    expect(
      screen.getByRole('spinbutton', {
        name: 'Maximum waiting requests per channel',
      })
    ).toHaveValue(0)
    expect(
      screen.getByText('0 automatically follows the channel concurrency limit.')
    ).toBeInTheDocument()
    expect(
      screen.getByRole('switch', { name: 'Enable concurrency cooldown' })
    ).toBeChecked()
    expect(
      screen.getByRole('spinbutton', { name: 'Cooldown duration (seconds)' })
    ).toHaveValue(30)
    expect(
      screen.getByRole('switch', {
        name: 'Cooldown on upstream HTTP 429',
      })
    ).toBeChecked()
    expect(
      screen.getByRole('switch', {
        name: 'Cooldown on fuzzy error message match',
      })
    ).not.toBeChecked()
    expect(
      screen.getByRole('switch', { name: 'Enable load snapshot cache' })
    ).toBeChecked()
    expect(
      screen.getByRole('spinbutton', { name: 'Slot TTL (minutes)' })
    ).toHaveValue(30)
    expect(
      screen.getByRole('spinbutton', {
        name: 'Wait polling interval (milliseconds)',
      })
    ).toHaveValue(100)
    expect(
      screen.getByRole('spinbutton', {
        name: 'Load snapshot cache TTL (milliseconds)',
      })
    ).toHaveValue(200)
    expect(
      screen.getByRole('spinbutton', {
        name: 'Maximum acquisition attempts per round',
      })
    ).toHaveValue(8)
    expect(
      await screen.findByRole('button', { name: 'Save Changes' })
    ).toBeEnabled()
  })

  it('explains each runtime control and the cache safety boundary', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <Fixture />
      </QueryClientProvider>
    )

    const descriptions = [
      'Configure waiting, cooldown, and load observation for channels with a concurrency limit.',
      'Maximum time a request waits for an available channel slot.',
      'Maximum queued requests per limited channel.',
      'How long a channel stays out of selection after a configured cooldown trigger.',
      'Triggers cooldown for explicit upstream HTTP 429 responses.',
      'Also matches rate-limit-like error text and may cause false positives; off by default.',
      'The cache only assists load observation; authoritative slot acquisition still enforces the limit. Its TTL remains visible and keeps its saved value while caching is off.',
      'Low-level lease, polling, cache, and acquisition safeguards.',
      'How long an acquired slot lease remains valid without renewal.',
      'How often waiting requests retry authoritative slot acquisition.',
      'How briefly load observations are reused for routing and status reads.',
      'Limits how many candidates one routing round may try to acquire.',
    ]

    for (const description of descriptions) {
      expect(screen.getByText(description)).toBeInTheDocument()
    }
  })

  it('disables dependent values without clearing them and restores editing', async () => {
    const user = userEvent.setup()
    render(
      <QueryClientProvider client={queryClient}>
        <Fixture />
      </QueryClientProvider>
    )

    const waitEnabled = screen.getByRole('switch', {
      name: 'Wait when channel concurrency is full',
    })
    const waitTimeout = screen.getByRole('spinbutton', {
      name: 'Wait timeout (milliseconds)',
    })
    const maximumWaiting = screen.getByRole('spinbutton', {
      name: 'Maximum waiting requests per channel',
    })
    const waitInterval = screen.getByRole('spinbutton', {
      name: 'Wait polling interval (milliseconds)',
    })
    await user.click(waitEnabled)
    expect(waitTimeout).toBeDisabled()
    expect(maximumWaiting).toBeDisabled()
    expect(waitInterval).toBeDisabled()
    expect(waitTimeout).toHaveValue(5000)
    expect(maximumWaiting).toHaveValue(0)
    expect(waitInterval).toHaveValue(100)
    await user.click(waitEnabled)
    expect(waitTimeout).toBeEnabled()
    expect(maximumWaiting).toBeEnabled()
    expect(waitInterval).toBeEnabled()

    const cooldownEnabled = screen.getByRole('switch', {
      name: 'Enable concurrency cooldown',
    })
    const cooldownDuration = screen.getByRole('spinbutton', {
      name: 'Cooldown duration (seconds)',
    })
    const cooldownOn429 = screen.getByRole('switch', {
      name: 'Cooldown on upstream HTTP 429',
    })
    const cooldownOnMessage = screen.getByRole('switch', {
      name: 'Cooldown on fuzzy error message match',
    })
    await user.click(cooldownEnabled)
    expect(cooldownDuration).toBeDisabled()
    expect(cooldownOn429).toHaveAttribute('aria-disabled', 'true')
    expect(cooldownOnMessage).toHaveAttribute('aria-disabled', 'true')
    expect(cooldownDuration).toHaveValue(30)
    expect(cooldownOn429).toBeChecked()
    expect(cooldownOnMessage).not.toBeChecked()
    await user.click(cooldownEnabled)
    expect(cooldownDuration).toBeEnabled()
    expect(cooldownOn429).not.toHaveAttribute('aria-disabled', 'true')
    expect(cooldownOnMessage).not.toHaveAttribute('aria-disabled', 'true')

    const cacheEnabled = screen.getByRole('switch', {
      name: 'Enable load snapshot cache',
    })
    const cacheTtl = screen.getByRole('spinbutton', {
      name: 'Load snapshot cache TTL (milliseconds)',
    })
    await user.click(cacheEnabled)
    expect(cacheTtl).toBeDisabled()
    expect(cacheTtl).toHaveValue(200)
    await user.click(cacheEnabled)
    expect(cacheTtl).toBeEnabled()

    expect(
      screen.getByRole('spinbutton', { name: 'Slot TTL (minutes)' })
    ).toBeEnabled()
    expect(
      screen.getByRole('spinbutton', {
        name: 'Maximum acquisition attempts per round',
      })
    ).toBeEnabled()
  })

  it.each([
    ['Wait timeout (milliseconds)', '0', 1, 60000],
    ['Wait timeout (milliseconds)', '1.5', 1, 60000],
    ['Maximum waiting requests per channel', '10001', 0, 10000],
    ['Cooldown duration (seconds)', '3601', 1, 3600],
    ['Slot TTL (minutes)', '0', 1, 1440],
    ['Wait polling interval (milliseconds)', '5001', 1, 5000],
    ['Load snapshot cache TTL (milliseconds)', '0', 1, 5000],
    ['Maximum acquisition attempts per round', '101', 1, 100],
  ])(
    'rejects invalid %s value %s before sending a request',
    async (label, value, minimum, maximum) => {
      const user = userEvent.setup()
      render(
        <QueryClientProvider client={queryClient}>
          <Fixture />
        </QueryClientProvider>
      )

      const input = screen.getByRole('spinbutton', { name: label })
      fireEvent.change(input, { target: { value } })
      await user.click(screen.getByRole('button', { name: 'Save Changes' }))

      const message = `Enter a whole number from ${minimum} to ${maximum}.`
      await waitFor(() => expect(input).toHaveAttribute('aria-invalid', 'true'))
      expect(input).toHaveAccessibleDescription(
        expect.stringContaining(message)
      )
      expect(api.put).not.toHaveBeenCalled()
    }
  )

  it('does not send requests when there are no changes', async () => {
    const user = userEvent.setup()
    const infoToast = vi.spyOn(toast, 'info')
    render(
      <QueryClientProvider client={queryClient}>
        <Fixture />
      </QueryClientProvider>
    )

    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    expect(infoToast).toHaveBeenCalledExactlyOnceWith('No changes to save')
    expect(api.put).not.toHaveBeenCalled()
  })

  it('saves only changed settings in the panel order before reporting success', async () => {
    const user = userEvent.setup()
    const successToast = vi.spyOn(toast, 'success')
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries')
    render(
      <QueryClientProvider client={queryClient}>
        <Fixture />
      </QueryClientProvider>
    )

    fireEvent.change(
      screen.getByRole('spinbutton', {
        name: 'Maximum acquisition attempts per round',
      }),
      { target: { value: '9' } }
    )
    fireEvent.change(
      screen.getByRole('spinbutton', { name: 'Wait timeout (milliseconds)' }),
      { target: { value: '7000' } }
    )
    await user.click(
      screen.getByRole('switch', { name: 'Enable load snapshot cache' })
    )
    await user.click(
      screen.getByRole('switch', {
        name: 'Wait when channel concurrency is full',
      })
    )
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(4))
    expect(api.put).toHaveBeenNthCalledWith(1, '/api/option/', {
      key: 'channel_concurrency_setting.wait_enabled',
      value: false,
    })
    expect(api.put).toHaveBeenNthCalledWith(2, '/api/option/', {
      key: 'channel_concurrency_setting.wait_timeout_ms',
      value: 7000,
    })
    expect(api.put).toHaveBeenNthCalledWith(3, '/api/option/', {
      key: 'channel_concurrency_setting.load_cache_enabled',
      value: false,
    })
    expect(api.put).toHaveBeenNthCalledWith(4, '/api/option/', {
      key: 'channel_concurrency_setting.max_acquire_attempts',
      value: 9,
    })
    expect(successToast).toHaveBeenCalledExactlyOnceWith(
      'Channel concurrency settings saved'
    )
    expect(invalidateQueries).toHaveBeenCalledExactlyOnceWith({
      queryKey: ['system-options'],
    })
  })

  it('stops after a failed update and reloads the actual server baseline', async () => {
    const user = userEvent.setup()
    const successToast = vi.spyOn(toast, 'success')
    const errorToast = vi.spyOn(toast, 'error')
    vi.mocked(api.put)
      .mockReset()
      .mockResolvedValueOnce({ data: { success: true } })
      .mockRejectedValueOnce(new Error('option update failed'))

    const renderPanel = (defaultValues: ChannelConcurrencySettingsValues) => (
      <QueryClientProvider client={queryClient}>
        <Fixture defaultValues={defaultValues} />
      </QueryClientProvider>
    )
    const { rerender } = render(renderPanel(defaults))

    fireEvent.change(
      screen.getByRole('spinbutton', { name: 'Wait timeout (milliseconds)' }),
      { target: { value: '7000' } }
    )
    fireEvent.change(
      screen.getByRole('spinbutton', {
        name: 'Maximum waiting requests per channel',
      }),
      { target: { value: '2' } }
    )
    await user.click(
      screen.getByRole('switch', {
        name: 'Wait when channel concurrency is full',
      })
    )
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(2))
    expect(api.put).toHaveBeenNthCalledWith(1, '/api/option/', {
      key: 'channel_concurrency_setting.wait_enabled',
      value: false,
    })
    expect(api.put).toHaveBeenNthCalledWith(2, '/api/option/', {
      key: 'channel_concurrency_setting.wait_timeout_ms',
      value: 7000,
    })
    expect(api.put).not.toHaveBeenCalledWith('/api/option/', {
      key: 'channel_concurrency_setting.max_waiting_per_channel',
      value: 2,
    })
    expect(successToast).not.toHaveBeenCalled()
    expect(errorToast).toHaveBeenCalledExactlyOnceWith(
      'Some channel concurrency settings may already be active. Later changes were not saved; server values are being reloaded.'
    )

    const serverAfterFailure = { ...defaults }
    serverAfterFailure['channel_concurrency_setting.wait_enabled'] = false
    rerender(renderPanel(serverAfterFailure))

    await waitFor(() =>
      expect(
        screen.getByRole('switch', {
          name: 'Wait when channel concurrency is full',
        })
      ).not.toBeChecked()
    )
    expect(
      screen.getByRole('spinbutton', { name: 'Wait timeout (milliseconds)' })
    ).toHaveValue(5000)
    expect(
      screen.getByRole('spinbutton', {
        name: 'Maximum waiting requests per channel',
      })
    ).toHaveValue(0)

    vi.mocked(api.put)
      .mockReset()
      .mockResolvedValue({
        data: { success: true },
      })
    await user.click(
      screen.getByRole('switch', {
        name: 'Wait when channel concurrency is full',
      })
    )
    fireEvent.change(
      screen.getByRole('spinbutton', { name: 'Wait timeout (milliseconds)' }),
      { target: { value: '7000' } }
    )
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))

    await waitFor(() => expect(api.put).toHaveBeenCalledTimes(2))
    expect(api.put).toHaveBeenNthCalledWith(1, '/api/option/', {
      key: 'channel_concurrency_setting.wait_enabled',
      value: true,
    })
    expect(api.put).toHaveBeenNthCalledWith(2, '/api/option/', {
      key: 'channel_concurrency_setting.wait_timeout_ms',
      value: 7000,
    })
    expect(successToast).toHaveBeenCalledExactlyOnceWith(
      'Channel concurrency settings saved'
    )
  })

  it('loads the Root-only panel through the operations route with missing-value defaults', async () => {
    vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
    vi.spyOn(api, 'get').mockImplementation(async (url) => {
      if (url === '/api/status') {
        return { data: { success: true, data: {} } }
      }
      if (url === '/api/option/') {
        return { data: { success: true, data: [] } }
      }
      throw new Error(`Unexpected GET ${url}`)
    })

    const rootRoute = createRootRoute()
    const authenticatedRoute = createRoute({
      getParentRoute: () => rootRoute,
      id: '_authenticated',
    })
    const operationsRoute = createRoute({
      getParentRoute: () => authenticatedRoute,
      path: '/system-settings/operations/$section',
      beforeLoad: OperationsRoute.options.beforeLoad as (context: {
        params: { section: string }
      }) => void,
      component: OperationsSettings,
    })
    const router = createRouter({
      routeTree: rootRoute.addChildren([
        authenticatedRoute.addChildren([operationsRoute]),
      ]),
      history: createMemoryHistory({
        initialEntries: ['/system-settings/operations/channel-concurrency'],
      }),
    })

    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )
    await act(() => router.load())

    expect(
      await screen.findByRole('heading', {
        name: 'Channel Concurrency Limits',
      })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('spinbutton', { name: 'Wait timeout (milliseconds)' })
    ).toHaveValue(5000)
    expect(
      screen.getByRole('spinbutton', { name: 'Slot TTL (minutes)' })
    ).toHaveValue(30)
  })
})
