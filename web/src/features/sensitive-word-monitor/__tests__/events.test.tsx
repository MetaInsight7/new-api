/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { cleanup, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import {
  renderSensitiveWordMonitor,
  sensitiveWordDashboard,
  sensitiveWordEventPage,
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

describe('sensitive-word event browsing', () => {
  it('applies server-side search, disposition, pagination, and custom time filters', async () => {
    const get = vi.spyOn(api, 'get').mockImplementation(async (url, config) => {
      if (url === '/api/sensitive-words/dashboard') {
        return {
          data: { success: true, data: sensitiveWordDashboard(24) },
        }
      }
      if (url === '/api/sensitive-words/events') {
        return {
          data: {
            success: true,
            data: sensitiveWordEventPage(undefined, {
              page: Number(config?.params?.p ?? 1),
              pageSize: Number(config?.params?.page_size ?? 20),
              total: 45,
            }),
          },
        }
      }
      throw new Error(`Unexpected GET ${url}`)
    })
    const user = userEvent.setup()

    await renderSensitiveWordMonitor()
    await screen.findByText('Recent sensitive-word events')
    await waitFor(() =>
      expect(get).toHaveBeenCalledWith('/api/sensitive-words/events', {
        params: {
          hours: 24,
          p: 1,
          page_size: 20,
          disposition: undefined,
          keyword: undefined,
        },
      })
    )

    const search = screen.getByRole('textbox', {
      name: 'Search sensitive-word events',
    })
    await user.type(search, 'alice')
    await user.click(screen.getByRole('button', { name: 'Search' }))
    await waitFor(() =>
      expect(get).toHaveBeenCalledWith('/api/sensitive-words/events', {
        params: expect.objectContaining({ p: 1, keyword: 'alice' }),
      })
    )

    await user.click(screen.getByRole('combobox', { name: 'Disposition' }))
    await user.click(screen.getByRole('option', { name: 'Blocked' }))
    await waitFor(() =>
      expect(get).toHaveBeenCalledWith('/api/sensitive-words/events', {
        params: expect.objectContaining({
          p: 1,
          disposition: 'block',
          keyword: 'alice',
        }),
      })
    )

    await user.click(screen.getByRole('button', { name: 'Go to next page' }))
    await waitFor(() =>
      expect(get).toHaveBeenCalledWith('/api/sensitive-words/events', {
        params: expect.objectContaining({
          p: 2,
          disposition: 'block',
          keyword: 'alice',
        }),
      })
    )

    await user.click(
      screen.getAllByRole('button', { name: 'Custom Time Range' })[0]
    )
    await user.click(screen.getByRole('button', { name: 'Apply Filters' }))
    await waitFor(() =>
      expect(get).toHaveBeenCalledWith('/api/sensitive-words/dashboard', {
        params: {
          start_timestamp: expect.any(Number),
          end_timestamp: expect.any(Number),
        },
      })
    )
    expect(get).toHaveBeenCalledWith('/api/sensitive-words/events', {
      params: {
        start_timestamp: expect.any(Number),
        end_timestamp: expect.any(Number),
        p: 1,
        page_size: 20,
        disposition: 'block',
        keyword: 'alice',
      },
    })
  })
})
