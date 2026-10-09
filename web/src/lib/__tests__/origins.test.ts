import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  normalizeOrigin,
  officialWebsiteUrl,
  resolveOfficialDocumentationUrl,
  resolveOfficialWebsiteOrigin,
} from '../origins'

afterEach(() => {
  vi.unstubAllEnvs()
  vi.resetModules()
})

describe('website origin configuration', () => {
  it('normalizes whitespace and trailing slashes from a configured origin', () => {
    expect(normalizeOrigin(' https://preview.example.test/// ')).toBe(
      'https://preview.example.test'
    )
    expect(
      officialWebsiteUrl('/models', 'https://preview.example.test///')
    ).toBe('https://preview.example.test/models')
  })

  it('defaults unset or blank website origins to the TokenDazi website', () => {
    expect(resolveOfficialWebsiteOrigin(undefined)).toBe(
      'https://tokendazi.com'
    )
    expect(resolveOfficialWebsiteOrigin('  ')).toBe('https://tokendazi.com')
  })

  it('uses a configured origin for canonical website paths', () => {
    expect(
      resolveOfficialWebsiteOrigin(' https://preview.example.test/ ')
    ).toBe('https://preview.example.test')
    expect(officialWebsiteUrl('rankings', 'https://preview.example.test')).toBe(
      'https://preview.example.test/rankings'
    )
  })

  it('retains the Overlay local-path fallback when explicitly given an empty origin', () => {
    expect(officialWebsiteUrl('/models', '')).toBe('/models')
  })

  it('uses the configured documentation URL or the brand documentation default', () => {
    expect(
      resolveOfficialDocumentationUrl(' https://docs.example.test/ ')
    ).toBe('https://docs.example.test/')
    expect(resolveOfficialDocumentationUrl(undefined)).toBe(
      'https://docs.tokendazi.com/'
    )
  })

  it('reads website and documentation overrides from the frontend build environment', async () => {
    vi.stubEnv(
      'VITE_OFFICIAL_WEBSITE_ORIGIN',
      ' https://preview.example.test/ '
    )
    vi.stubEnv('VITE_OFFICIAL_DOCUMENTATION_URL', 'https://docs.example.test/')
    vi.resetModules()

    const configuredOrigins = await import('../origins')

    expect(configuredOrigins.OFFICIAL_WEBSITE_ORIGIN).toBe(
      'https://preview.example.test'
    )
    expect(configuredOrigins.officialWebsiteUrl('/models')).toBe(
      'https://preview.example.test/models'
    )
    expect(configuredOrigins.OFFICIAL_DOCUMENTATION_URL).toBe(
      'https://docs.example.test/'
    )
  })
})
