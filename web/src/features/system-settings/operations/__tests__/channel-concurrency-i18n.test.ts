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
  'Channel Concurrency Limits',
  'Configure waiting, cooldown, and load observation for channels with a concurrency limit.',
  'Common policies',
  'Frequently adjusted waiting, cooldown, and cache controls.',
  'Wait when channel concurrency is full',
  'When waiting is off, its dependent values remain visible and keep their saved values.',
  'Wait timeout (milliseconds)',
  'Maximum time a request waits for an available channel slot.',
  'Maximum waiting requests per channel',
  'Maximum queued requests per limited channel.',
  '0 automatically follows the channel concurrency limit.',
  'Enable concurrency cooldown',
  'When cooldown is off, its duration and triggers remain visible and keep their saved values.',
  'Cooldown duration (seconds)',
  'How long a channel stays out of selection after a configured cooldown trigger.',
  'Cooldown on upstream HTTP 429',
  'Triggers cooldown for explicit upstream HTTP 429 responses.',
  'Cooldown on fuzzy error message match',
  'Also matches rate-limit-like error text and may cause false positives; off by default.',
  'Enable load snapshot cache',
  'The cache only assists load observation; authoritative slot acquisition still enforces the limit. Its TTL remains visible and keeps its saved value while caching is off.',
  'Advanced runtime parameters',
  'Low-level lease, polling, cache, and acquisition safeguards.',
  'Slot TTL (minutes)',
  'How long an acquired slot lease remains valid without renewal.',
  'Wait polling interval (milliseconds)',
  'How often waiting requests retry authoritative slot acquisition.',
  'Load snapshot cache TTL (milliseconds)',
  'How briefly load observations are reused for routing and status reads.',
  'Maximum acquisition attempts per round',
  'Limits how many candidates one routing round may try to acquire.',
  'Enter a whole number from {{minimum}} to {{maximum}}.',
  'No changes to save',
  'Channel concurrency settings saved',
  'Some channel concurrency settings may already be active. Later changes were not saved; server values are being reloaded.',
] as const

const locales = { en, zh, 'zh-TW': zhTW, fr, ru, ja, vi }

describe('channel concurrency settings translations', () => {
  it.each(Object.entries(locales))(
    'defines every panel key in %s without fallback',
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

  it('keeps every panel key in the static extraction list', () => {
    for (const key of requiredKeys) {
      expect(STATIC_I18N_KEYS).toContain(key)
    }
  })
})
