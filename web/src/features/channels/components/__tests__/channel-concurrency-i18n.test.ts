import { describe, expect, it } from 'vitest'

import en from '@/i18n/locales/en.json'
import fr from '@/i18n/locales/fr.json'
import ja from '@/i18n/locales/ja.json'
import ru from '@/i18n/locales/ru.json'
import vi from '@/i18n/locales/vi.json'
import zhTW from '@/i18n/locales/zh-TW.json'
import zh from '@/i18n/locales/zh.json'
import { STATIC_I18N_KEYS } from '@/i18n/static-keys'

const requiredKeys = [
  'Concurrency',
  'Channel concurrency unavailable',
  'Channel concurrency: {{active}} of {{max}} active',
  '{{count}} waiting',
  'Cooling',
  'Maximum Concurrency',
  'Maximum simultaneous upstream requests for this channel; 0 means unlimited.',
  'Maximum concurrency must be a non-negative integer.',
  'Maximum concurrency saved',
  'Failed to save maximum concurrency. Try again.',
] as const

const locales = { en, zh, 'zh-TW': zhTW, fr, ru, ja, vi }

describe('channel concurrency translations', () => {
  it.each(Object.entries(locales))(
    'defines every key in %s without fallback',
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

  it('keeps every key in the static extraction list', () => {
    for (const key of requiredKeys) {
      expect(STATIC_I18N_KEYS).toContain(key)
    }
  })
})
