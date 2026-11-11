/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, it } from 'vitest'

import en from '@/i18n/locales/en.json'
import fr from '@/i18n/locales/fr.json'
import ja from '@/i18n/locales/ja.json'
import ru from '@/i18n/locales/ru.json'
import vi from '@/i18n/locales/vi.json'
import zhTW from '@/i18n/locales/zh-TW.json'
import zh from '@/i18n/locales/zh.json'

import { renderAuditContent } from '../format'

const requiredKeys = [
  'Updated sensitive-word safety state for user {{target_user_id}}',
  'Unbanned user {{target_user_id}} and reset sensitive-word violations',
  'Invalid audit ID',
  'Invalid rule ID',
  'Sensitive word cannot be empty',
  'Sensitive word cannot exceed {{max}} characters',
  'A sensitive-word rule must contain at least one valid entry',
  'A single rule cannot contain more than {{max}} sensitive words',
  'Scoped sensitive-word rules cannot target the automatic group',
  'Group {{group}} is not configured for pricing',
  'Sensitive-word policy storage is unavailable; complete the database migration first',
  'Sensitive-word policy transaction is unavailable',
  'Rule name is required',
  'Rule name cannot exceed 64 characters',
  'Rule must contain at least one sensitive word',
  'Invalid rule scope',
  'Scoped rules must select at least one group',
  'Invalid rule mode',
] as const

const locales = { en, zh, 'zh-TW': zhTW, fr, ru, ja, vi }

function interpolate(
  key: string,
  params: Record<string, unknown> = {}
): string {
  return key.replaceAll(/\{\{([^}]+)}}/g, (placeholder, name: string) =>
    Object.hasOwn(params, name) ? String(params[name]) : placeholder
  )
}

describe('sensitive-word audit mappings', () => {
  it.each([
    [
      'sensitive_word.user_update',
      'Updated sensitive-word safety state for user 42',
    ],
    [
      'sensitive_word.unban',
      'Unbanned user 42 and reset sensitive-word violations',
    ],
  ])('renders %s through its localized action template', (action, expected) => {
    expect(
      renderAuditContent(
        { op: { action, params: { target_user_id: 42 } } },
        interpolate
      )
    ).toBe(expected)
  })

  it.each(Object.entries(locales))(
    'defines every management message in %s without fallback',
    (locale, resources) => {
      for (const key of requiredKeys) {
        expect(resources.translation).toHaveProperty(key)
        const value = resources.translation[key]
        expect(value.trim()).not.toBe('')
        if (locale !== 'en') {
          expect(value).not.toBe(key)
        }
      }
    }
  )
})
