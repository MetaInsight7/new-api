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

import type { SensitiveWordMonitorWindow } from '../types'
import {
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

describe('sensitive-word monitor dashboard states', () => {
  it('exposes an accessible loading state while the dashboard is pending', async () => {
    vi.spyOn(api, 'get').mockImplementation(() => new Promise(() => {}))

    await renderSensitiveWordMonitor()

    expect(screen.getByLabelText('Loading')).toBeVisible()
  })

  it('requests and renders the selected dashboard window', async () => {
    const get = vi.spyOn(api, 'get').mockImplementation(async (url, config) => {
      expect(url).toBe('/api/sensitive-words/dashboard')
      const hours = Number(config?.params?.hours) as SensitiveWordMonitorWindow
      return { data: { success: true, data: sensitiveWordDashboard(hours) } }
    })

    await renderSensitiveWordMonitor()
    expect(await screen.findByText('Sensitive-word monitor')).toBeVisible()
    expect((await screen.findAllByText('Credential abuse'))[0]).toBeVisible()
    expect(screen.getAllByText('gpt-test')[0]).toBeVisible()
    expect(get).toHaveBeenCalledWith('/api/sensitive-words/dashboard', {
      params: { hours: 24 },
    })

    await userEvent.click(screen.getByRole('button', { name: '7d' }))
    await waitFor(() =>
      expect(get).toHaveBeenCalledWith('/api/sensitive-words/dashboard', {
        params: { hours: 168 },
      })
    )
    expect((await screen.findAllByText('168'))[0]).toBeVisible()
  })

  it('recovers after the dashboard request fails and the user retries', async () => {
    const get = vi
      .spyOn(api, 'get')
      .mockRejectedValueOnce(new Error('dashboard unavailable'))
      .mockResolvedValue({
        data: { success: true, data: sensitiveWordDashboard(24) },
      })

    await renderSensitiveWordMonitor()
    expect(
      await screen.findByText('Unable to load sensitive-word monitor')
    ).toBeVisible()

    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))

    expect(await screen.findByText('Sensitive-word monitor')).toBeVisible()
    expect((await screen.findAllByText('Credential abuse'))[0]).toBeVisible()
    expect(get).toHaveBeenCalledTimes(2)
  })

  it('renders empty dashboard collections without failing', async () => {
    const dashboard = sensitiveWordDashboard(24)
    dashboard.series = []
    dashboard.top_rules = []
    dashboard.top_users = []
    dashboard.top_models = []
    dashboard.top_groups = []
    dashboard.recent_events = []
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: dashboard },
    })

    await renderSensitiveWordMonitor()

    expect(await screen.findByText('Sensitive-word monitor')).toBeVisible()
    expect(
      await screen.findAllByText('No matches in this window')
    ).toHaveLength(4)
    expect(screen.getByText('No sensitive-word events')).toBeVisible()
  })
})
