/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { act, cleanup, screen, waitFor } from '@testing-library/react'
import i18next from 'i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { INTERFACE_LANGUAGE_OPTIONS, toIntlLocale } from '@/i18n/languages'
import { api } from '@/lib/api'
import { formatNumber } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

import {
  renderSensitiveWordMonitor,
  sensitiveWordDashboard,
  setSensitiveWordManager,
} from './test-utils'

const languageCases = [
  ...INTERFACE_LANGUAGE_OPTIONS.map((language) => language.code),
  'invalid_language_code',
]

beforeEach(async () => {
  await i18next.changeLanguage('en')
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  setSensitiveWordManager()
})

afterEach(async () => {
  cleanup()
  useAuthStore.getState().auth.reset()
  vi.restoreAllMocks()
  await i18next.changeLanguage('en')
})

describe('sensitive-word monitor number formatting', () => {
  it.each(languageCases)(
    'formats values for %s through a valid Intl locale or runtime fallback',
    (language) => {
      const locale = toIntlLocale(language)
      const expected = Intl.NumberFormat(locale, {
        maximumFractionDigits: 2,
      }).format(1_234_567)

      expect(formatNumber(1_234_567, locale)).toBe(expected)
      if (language === 'invalid_language_code') expect(locale).toBeUndefined()
    }
  )

  it('updates rendered user identifiers when the interface language changes', async () => {
    const dashboard = sensitiveWordDashboard(24)
    dashboard.recent_events[0].username = ''
    dashboard.recent_events[0].user_id = 1_234_567
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: dashboard },
    })

    await renderSensitiveWordMonitor()
    const englishID = `#${formatNumber(1_234_567, toIntlLocale('en'))}`
    expect(await screen.findByText(englishID)).toBeVisible()

    await act(() => i18next.changeLanguage('fr'))

    const frenchID = `#${formatNumber(1_234_567, toIntlLocale('fr'))}`
    await waitFor(() => expect(document.body.textContent).toContain(frenchID))
    expect(frenchID).not.toBe(englishID)
  })
})
