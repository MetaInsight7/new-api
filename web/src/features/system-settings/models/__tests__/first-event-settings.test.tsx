import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { useState } from 'react'
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import zh from '@/i18n/locales/zh.json'
import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { GlobalSettingsCard } from '../global-settings-card'

const defaults = {
  global: {
    pass_through_request_enabled: false,
    thinking_model_blacklist: '[]',
    chat_completions_to_responses_policy: '{}',
  },
  general_setting: { ping_interval_enabled: false, ping_interval_seconds: 60 },
  streaming_setting: {
    first_response_timeout_enabled: false,
    first_response_timeout_seconds: 600,
  },
}

function Fixture(props: { defaultValues?: typeof defaults }) {
  const [actionsContainer, setActionsContainer] =
    useState<HTMLDivElement | null>(null)
  return (
    <>
      <div ref={setActionsContainer} />
      <SettingsPageProvider
        actionsContainer={actionsContainer}
        suppressSectionHeader={false}
      >
        <GlobalSettingsCard defaultValues={props.defaultValues ?? defaults} />
      </SettingsPageProvider>
    </>
  )
}

describe('first-event protection settings', () => {
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

  afterEach(async () => {
    await i18next.changeLanguage('en')
    i18next.removeResourceBundle('zh', 'translation')
    queryClient.clear()
  })

  it('defaults off with a preserved disabled 600-second budget independent of keep-alive', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <Fixture />
      </QueryClientProvider>
    )
    expect(
      screen.getByRole('switch', { name: 'First-event timeout protection' })
    ).not.toBeChecked()
    expect(
      screen.getByRole('spinbutton', { name: 'First-event wait (seconds)' })
    ).toBeDisabled()
    expect(
      screen.getByRole('spinbutton', { name: 'First-event wait (seconds)' })
    ).toHaveValue(600)
  })

  it('retains the duration through keyboard toggling without enabling keep-alive', async () => {
    const user = userEvent.setup()
    render(
      <QueryClientProvider client={queryClient}>
        <Fixture />
      </QueryClientProvider>
    )
    const protection = screen.getByRole('switch', {
      name: 'First-event timeout protection',
    })
    protection.focus()
    await user.keyboard('[Space]')
    const duration = screen.getByRole('spinbutton', {
      name: 'First-event wait (seconds)',
    })
    expect(duration).toBeEnabled()
    fireEvent.change(duration, { target: { value: '1200' } })
    await user.click(protection)
    expect(duration).toBeDisabled()
    expect(duration).toHaveValue(1200)
    await user.click(protection)
    expect(duration).toHaveValue(1200)
    expect(
      screen.getByRole('switch', { name: 'Keep-alive Ping' })
    ).not.toBeChecked()
  })

  it('saves the duration before enabling protection and shows success only after both saves', async () => {
    const user = userEvent.setup()
    const success = vi.spyOn(toast, 'success')
    render(
      <QueryClientProvider client={queryClient}>
        <Fixture />
      </QueryClientProvider>
    )
    await user.click(
      screen.getByRole('switch', { name: 'First-event timeout protection' })
    )
    fireEvent.change(
      screen.getByRole('spinbutton', { name: 'First-event wait (seconds)' }),
      { target: { value: '1200' } }
    )
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() =>
      expect(success).toHaveBeenCalledWith('Setting updated successfully')
    )
    expect(vi.mocked(api.put).mock.calls.map((call) => call[1])).toEqual([
      { key: 'streaming_setting.first_response_timeout_seconds', value: 1200 },
      { key: 'streaming_setting.first_response_timeout_enabled', value: true },
    ])
  })

  it('does not enable protection or report success when saving the duration fails', async () => {
    const user = userEvent.setup()
    vi.mocked(api.put).mockResolvedValueOnce({
      data: { success: false, message: 'settings storage unavailable' },
    })
    const success = vi.spyOn(toast, 'success')
    const failure = vi.spyOn(toast, 'error')
    render(
      <QueryClientProvider client={queryClient}>
        <Fixture />
      </QueryClientProvider>
    )
    await user.click(
      screen.getByRole('switch', { name: 'First-event timeout protection' })
    )
    fireEvent.change(
      screen.getByRole('spinbutton', { name: 'First-event wait (seconds)' }),
      { target: { value: '1200' } }
    )
    await user.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(failure).toHaveBeenCalled())
    expect(api.put).toHaveBeenCalledTimes(1)
    expect(success).not.toHaveBeenCalled()
  })

  it.each(['success', 'failure'] as const)(
    'keeps Save disabled across sequential pending writes and recovers after %s',
    async (outcome) => {
      const user = userEvent.setup()
      let resolveDurationSave!: (value: { data: { success: boolean } }) => void
      const durationSave = new Promise<{ data: { success: boolean } }>(
        (resolve) => {
          resolveDurationSave = resolve
        }
      )
      let resolveEnableSave!: (value: {
        data: { success: boolean; message: string }
      }) => void
      const enableSave = new Promise<{
        data: { success: boolean; message: string }
      }>((resolve) => {
        resolveEnableSave = resolve
      })
      vi.mocked(api.put)
        .mockImplementationOnce(async () => durationSave)
        .mockImplementationOnce(async () => enableSave)
      const success = vi.spyOn(toast, 'success')
      const failure = vi.spyOn(toast, 'error')
      render(
        <QueryClientProvider client={queryClient}>
          <Fixture />
        </QueryClientProvider>
      )
      await user.click(
        screen.getByRole('switch', { name: 'First-event timeout protection' })
      )
      fireEvent.change(
        screen.getByRole('spinbutton', { name: 'First-event wait (seconds)' }),
        { target: { value: '1200' } }
      )
      await user.click(screen.getByRole('button', { name: 'Save Changes' }))
      expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled()
      expect(success).not.toHaveBeenCalled()

      await act(async () => {
        resolveDurationSave({ data: { success: true } })
      })
      await waitFor(() => expect(api.put).toHaveBeenCalledTimes(2))
      expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled()
      expect(success).not.toHaveBeenCalled()

      await act(async () => {
        resolveEnableSave({
          data: {
            success: outcome === 'success',
            message: 'enable save failed',
          },
        })
      })
      await waitFor(() =>
        expect(
          screen.getByRole('button', { name: 'Save Changes' })
        ).toBeEnabled()
      )
      if (outcome === 'success') {
        expect(success).toHaveBeenCalledWith('Setting updated successfully')
        expect(failure).not.toHaveBeenCalled()
      } else {
        expect(success).not.toHaveBeenCalled()
        expect(failure).toHaveBeenCalled()
      }
    }
  )

  it.each(['0', '-1', '1.5', '9223372037', ''])(
    'rejects invalid seconds %s with an accessible field error',
    async (value) => {
      const user = userEvent.setup()
      render(
        <QueryClientProvider client={queryClient}>
          <Fixture />
        </QueryClientProvider>
      )
      await user.click(
        screen.getByRole('switch', { name: 'First-event timeout protection' })
      )
      const duration = screen.getByRole('spinbutton', {
        name: 'First-event wait (seconds)',
      })
      fireEvent.change(duration, { target: { value } })
      await user.click(screen.getByRole('button', { name: 'Save Changes' }))
      expect(
        await screen.findByText(
          'First-event wait must be an integer from 1 to 9223372036 seconds.'
        )
      ).toBeInTheDocument()
      expect(duration).toHaveAttribute('aria-invalid', 'true')
      expect(api.put).not.toHaveBeenCalled()
    }
  )

  it('shows the last saved server values after a settings refresh', () => {
    const view = render(
      <QueryClientProvider client={queryClient}>
        <Fixture />
      </QueryClientProvider>
    )
    view.rerender(
      <QueryClientProvider client={queryClient}>
        <Fixture
          defaultValues={{
            ...defaults,
            streaming_setting: {
              first_response_timeout_enabled: true,
              first_response_timeout_seconds: 900,
            },
          }}
        />
      </QueryClientProvider>
    )
    expect(
      screen.getByRole('switch', { name: 'First-event timeout protection' })
    ).toBeChecked()
    expect(
      screen.getByRole('spinbutton', { name: 'First-event wait (seconds)' })
    ).toHaveValue(900)
  })

  it('translates labels and validation while preserving the budget across language changes', async () => {
    const user = userEvent.setup()
    render(
      <QueryClientProvider client={queryClient}>
        <Fixture />
      </QueryClientProvider>
    )
    await act(async () => {
      i18next.addResourceBundle('zh', 'translation', zh.translation)
      await i18next.changeLanguage('zh')
    })
    await user.click(screen.getByRole('switch', { name: '首事件超时保护' }))
    const duration = screen.getByRole('spinbutton', {
      name: '首事件等待时长（秒）',
    })
    fireEvent.change(duration, { target: { value: '0' } })
    await user.click(screen.getByRole('button', { name: '保存更改' }))
    expect(
      await screen.findByText(
        '首事件等待时长必须是 1 到 9223372036 秒之间的整数。'
      )
    ).toBeInTheDocument()
    await act(async () => {
      await i18next.changeLanguage('en')
    })
    expect(
      screen.getByRole('switch', { name: 'First-event timeout protection' })
    ).toBeChecked()
    expect(
      screen.getByRole('spinbutton', { name: 'First-event wait (seconds)' })
    ).toHaveValue(0)
  })
})
