/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { cleanup, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import type { SensitiveWordMonitorWindow } from '../types'
import {
  affectedSensitiveWordUser,
  renderSensitiveWordMonitor,
  sensitiveWordDashboard,
  setSensitiveWordManager,
} from './test-utils'

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  setSensitiveWordManager()
})

afterEach(() => {
  cleanup()
  useAuthStore.getState().auth.reset()
  vi.restoreAllMocks()
})

describe('sensitive-word event account review', () => {
  it('keeps account handling available when evidence metadata is redacted', async () => {
    const dashboard = sensitiveWordDashboard(24)
    dashboard.sensitive_metadata_included = false
    dashboard.top_rules = []
    dashboard.recent_events = dashboard.recent_events.map(
      ({ audit_id: _auditID, rule_names: _ruleNames, ...event }) => event
    )
    setSensitiveWordManager({ evidenceRead: false })
    const get = vi.spyOn(api, 'get').mockImplementation(async (url) => {
      if (url === '/api/sensitive-words/dashboard') {
        return { data: { success: true, data: dashboard } }
      }
      if (url === '/api/user/7') {
        return {
          data: { success: true, data: affectedSensitiveWordUser() },
        }
      }
      throw new Error(`Unexpected GET ${url}`)
    })

    await renderSensitiveWordMonitor()

    expect(
      await screen.findByText('Recent sensitive-word events')
    ).toBeVisible()
    expect(screen.queryByText('Top matched rules')).not.toBeInTheDocument()
    expect(screen.queryByText('Credential abuse')).not.toBeInTheDocument()
    await userEvent.click(
      (await screen.findAllByRole('button', { name: 'Review and handle' }))[0]
    )
    expect(await screen.findByText('Account handling')).toBeVisible()
    expect(
      screen.getByText('No permission to view this audit evidence')
    ).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Unban and clear count' })
    ).toBeVisible()
    expect(get).toHaveBeenCalledWith('/api/user/7')
    expect(
      get.mock.calls.some(([url]) =>
        String(url).startsWith('/api/log/sensitive-word-audit/')
      )
    ).toBe(false)
  })

  it('does not offer sensitive-word unban for another disable source', async () => {
    setSensitiveWordManager({ evidenceRead: false })
    vi.spyOn(api, 'get').mockImplementation(async (url) => {
      if (url === '/api/sensitive-words/dashboard') {
        return { data: { success: true, data: sensitiveWordDashboard(24) } }
      }
      if (url === '/api/user/7') {
        return {
          data: {
            success: true,
            data: {
              ...affectedSensitiveWordUser(),
              sensitive_word_auto_banned: false,
            },
          },
        }
      }
      throw new Error(`Unexpected GET ${url}`)
    })

    await renderSensitiveWordMonitor()
    await userEvent.click(
      (await screen.findAllByRole('button', { name: 'Review and handle' }))[0]
    )

    expect(await screen.findByText('Account handling')).toBeVisible()
    expect(
      screen.queryByRole('button', { name: 'Unban and clear count' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Clear count' })).toBeVisible()
  })

  it('shows an inline error when the affected user cannot be loaded', async () => {
    setSensitiveWordManager({ evidenceRead: false })
    vi.spyOn(api, 'get').mockImplementation(async (url) => {
      if (url === '/api/sensitive-words/dashboard') {
        return { data: { success: true, data: sensitiveWordDashboard(24) } }
      }
      if (url === '/api/user/7') {
        throw new Error('user lookup unavailable')
      }
      throw new Error(`Unexpected GET ${url}`)
    })

    await renderSensitiveWordMonitor()
    await userEvent.click(
      (await screen.findAllByRole('button', { name: 'Review and handle' }))[0]
    )

    expect(
      await screen.findByText('Unable to load affected user')
    ).toBeVisible()
    expect(screen.getByText('Refresh the panel and try again.')).toBeVisible()
  })

  it('reports account update failures and keeps the action available', async () => {
    setSensitiveWordManager({ evidenceRead: false })
    vi.spyOn(api, 'get').mockImplementation(async (url) => {
      if (url === '/api/sensitive-words/dashboard') {
        return { data: { success: true, data: sensitiveWordDashboard(24) } }
      }
      if (url === '/api/user/7') {
        return {
          data: { success: true, data: affectedSensitiveWordUser() },
        }
      }
      throw new Error(`Unexpected GET ${url}`)
    })
    vi.spyOn(api, 'patch').mockRejectedValue(new Error('update unavailable'))
    const errorToast = vi
      .spyOn(toast, 'error')
      .mockReturnValue('sensitive-word-update-error')

    await renderSensitiveWordMonitor()
    await userEvent.click(
      (await screen.findAllByRole('button', { name: 'Review and handle' }))[0]
    )
    await userEvent.click(
      await screen.findByRole('button', { name: 'Add to whitelist' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Confirm' }))

    await waitFor(() => expect(errorToast).toHaveBeenCalled())
    expect(screen.getByRole('button', { name: 'Confirm' })).toBeEnabled()
  })

  it('reviews retained context and updates the affected account in place', async () => {
    const get = vi.spyOn(api, 'get').mockImplementation(async (url, config) => {
      if (url === '/api/sensitive-words/dashboard') {
        const hours = Number(
          config?.params?.hours
        ) as SensitiveWordMonitorWindow
        return {
          data: { success: true, data: sensitiveWordDashboard(hours) },
        }
      }
      if (url === '/api/user/7') {
        return {
          data: { success: true, data: affectedSensitiveWordUser() },
        }
      }
      if (url === '/api/log/sensitive-word-audit/10') {
        return {
          data: {
            success: true,
            data: {
              id: 10,
              request_id: 'request-sensitive-10',
              blocked: true,
              violation_count: 3,
              matched_words: '["blocked-word"]',
              matched_rule_names: '["Credential abuse"]',
              matched_rule_ids: '[1]',
              evidence: JSON.stringify({
                version: 1,
                truncated: false,
                matches: [],
                fields: [
                  {
                    role: 'system',
                    kind: 'content',
                    round: -1,
                    order: 0,
                    text: 'retained system context',
                  },
                  {
                    role: 'user',
                    kind: 'content',
                    round: 2,
                    order: 1,
                    text: 'retained matched round',
                  },
                ],
              }),
            },
          },
        }
      }
      throw new Error(`Unexpected GET ${url}`)
    })
    const patch = vi
      .spyOn(api, 'patch')
      .mockResolvedValue({ data: { success: true } })

    await renderSensitiveWordMonitor()
    await userEvent.click(
      (await screen.findAllByRole('button', { name: 'Review and handle' }))[0]
    )

    expect(await screen.findByText('retained system context')).toBeVisible()
    expect(screen.getByText('retained matched round')).toBeVisible()
    expect(screen.getByText('Round 3')).toBeVisible()
    expect(screen.getByText('Account handling')).toBeVisible()
    expect(get).toHaveBeenCalledWith('/api/log/sensitive-word-audit/10')
    const logLink = screen.getByText('View full log').closest('a')
    expect(logLink).not.toBeNull()
    expect(logLink).toHaveAttribute(
      'href',
      expect.stringContaining('requestId=request-sensitive-10')
    )

    await userEvent.click(
      screen.getByRole('button', { name: 'Add to whitelist' })
    )
    expect(
      screen.getByText(
        'Add alice to the sensitive-word whitelist? Future matches will still be audited, but requests will be allowed and violations will not increase.'
      )
    ).toBeVisible()
    await userEvent.click(screen.getByRole('button', { name: 'Confirm' }))
    await waitFor(() =>
      expect(patch).toHaveBeenCalledWith('/api/sensitive-words/users/7', {
        whitelist: true,
      })
    )
  })
})
