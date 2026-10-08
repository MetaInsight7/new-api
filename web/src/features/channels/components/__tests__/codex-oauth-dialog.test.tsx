import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { CodexOAuthDialog } from '../dialogs/codex-oauth-dialog'

const authorization = {
  data: {
    success: true,
    data: {
      authorize_url: 'https://auth.openai.com/oauth/authorize?state=test',
      expires_at: 2000000000,
    },
  },
}

describe('Codex channel authorization', () => {
  test('creates a link and returns the new channel credential after pasted callback', async () => {
    vi.spyOn(api, 'post')
      .mockResolvedValueOnce(authorization)
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: { key: '{"access_token":"credential"}', account_id: 'account' },
        },
      })
    const authorized = vi.fn()
    render(
      <CodexOAuthDialog open onOpenChange={vi.fn()} onAuthorized={authorized} />
    )
    const link = await screen.findByRole('link', {
      name: 'Open authorization page',
    })
    expect(link).toHaveAttribute('href', authorization.data.data.authorize_url)
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    const user = userEvent.setup()
    await user.type(
      screen.getByLabelText('Authorization callback'),
      'code#test'
    )
    await user.click(
      screen.getByRole('button', { name: 'Generate credential' })
    )
    await waitFor(() =>
      expect(authorized).toHaveBeenCalledWith(
        expect.objectContaining({ key: '{"access_token":"credential"}' })
      )
    )
  })

  test('does not accept a business failure as successful authorization', async () => {
    vi.spyOn(api, 'post')
      .mockResolvedValueOnce(authorization)
      .mockResolvedValueOnce({
        data: { success: false, message: 'Authorization expired' },
      })
    const authorized = vi.fn()
    render(
      <CodexOAuthDialog
        open
        channelId={9}
        onOpenChange={vi.fn()}
        onAuthorized={authorized}
      />
    )
    await screen.findByRole('link', { name: 'Open authorization page' })
    const user = userEvent.setup()
    await user.type(
      screen.getByLabelText('Authorization callback'),
      'code#test'
    )
    await user.click(
      screen.getByRole('button', { name: 'Save authorized credential' })
    )
    expect(await screen.findByText('Authorization expired')).toBeVisible()
    expect(authorized).not.toHaveBeenCalled()
    expect(
      screen.getByRole('button', { name: 'Restart authorization' })
    ).toBeEnabled()
  })

  test('ignores completion after closing the dialog or switching channels', async () => {
    let finish: (value: unknown) => void = () => {}
    vi.spyOn(api, 'post')
      .mockResolvedValueOnce(authorization)
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finish = resolve
          })
      )
      .mockResolvedValue(authorization)
    const authorized = vi.fn()
    const props = {
      open: true,
      channelId: 9,
      onOpenChange: vi.fn(),
      onAuthorized: authorized,
    }
    const view = render(<CodexOAuthDialog {...props} />)
    await screen.findByRole('link', { name: 'Open authorization page' })
    const user = userEvent.setup()
    await user.type(
      screen.getByLabelText('Authorization callback'),
      'code#test'
    )
    await user.click(
      screen.getByRole('button', { name: 'Save authorized credential' })
    )
    view.rerender(<CodexOAuthDialog {...props} open={false} channelId={10} />)
    await act(async () =>
      finish({ data: { success: true, data: { account_id: 'old-account' } } })
    )
    expect(authorized).not.toHaveBeenCalled()
  })
})
