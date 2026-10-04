/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

const ATTRIBUTION_STORAGE_KEY = 'newapi:ads-attribution'
const ATTRIBUTION_TTL_MS = 90 * 24 * 60 * 60 * 1000
const MAX_VALUE_LENGTH = 256
const MAX_PAYLOAD_LENGTH = 3500

const ATTRIBUTION_KEYS = new Set([
  'aff',
  'fbclid',
  'gad_campaignid',
  'gad_source',
  'gbraid',
  'gclid',
  'hsa_kw',
  'hsa_mt',
  'lng',
  'msclkid',
  'ttclid',
  'utm_campaign',
  'utm_content',
  'utm_medium',
  'utm_source',
  'utm_term',
  'wbraid',
  'yclid',
])

type AttributionValues = Record<string, string>

function isAttributionKey(key: string): boolean {
  return (
    ATTRIBUTION_KEYS.has(key) ||
    key.startsWith('utm_') ||
    key.startsWith('hsa_')
  )
}

function isAcquisitionPath(path: string): boolean {
  return (
    path.startsWith('/') &&
    path !== '/sign-in' &&
    path !== '/sign-up' &&
    !path.startsWith('/oauth/')
  )
}

function cleanValues(values: AttributionValues): AttributionValues {
  return Object.fromEntries(
    Object.entries(values)
      .map(([key, value]) => [
        key.trim().slice(0, 64),
        value.trim().slice(0, MAX_VALUE_LENGTH),
      ])
      .filter(([key, value]) => key && value)
  )
}

function readStoredAttribution(): AttributionValues {
  if (typeof window === 'undefined') return {}
  try {
    const raw = window.localStorage.getItem(ATTRIBUTION_STORAGE_KEY)
    if (!raw) return {}
    const values = JSON.parse(raw) as unknown
    if (!values || typeof values !== 'object' || Array.isArray(values)) {
      return {}
    }
    const cleaned = cleanValues(values as AttributionValues)
    const expiresAt = Date.parse(cleaned.expires_at ?? '')
    return Number.isFinite(expiresAt) && expiresAt <= Date.now() ? {} : cleaned
  } catch {
    return {}
  }
}

function sanitizedExternalReferrer(referrer: string): string {
  if (!referrer || typeof window === 'undefined') return ''
  try {
    const value = new URL(referrer)
    if (value.origin === window.location.origin) return ''
    return `${value.origin}${value.pathname}`.slice(0, MAX_VALUE_LENGTH)
  } catch {
    return ''
  }
}

function serializeAttribution(values: AttributionValues): string {
  const cleaned = cleanValues(values)
  let result = JSON.stringify(cleaned)
  if (result.length <= MAX_PAYLOAD_LENGTH) return result

  for (const key of ['referrer', 'first_landing_path', 'landing_path']) {
    if (result.length <= MAX_PAYLOAD_LENGTH) break
    delete cleaned[key]
    result = JSON.stringify(cleaned)
  }
  return result.length <= MAX_PAYLOAD_LENGTH ? result : ''
}

export function captureAttribution(): void {
  if (typeof window === 'undefined') return

  const existing = readStoredAttribution()
  const current: AttributionValues = {}
  const search = new URLSearchParams(window.location.search)
  for (const [rawKey, value] of search.entries()) {
    const key = rawKey.trim().toLowerCase()
    if (isAttributionKey(key) && value.trim()) {
      current[key] = value
    }
  }

  const landingPath = isAcquisitionPath(window.location.pathname)
    ? window.location.pathname
    : ''
  const referrer = sanitizedExternalReferrer(document.referrer)
  let affiliate = ''
  try {
    affiliate = window.localStorage.getItem('aff')?.trim() ?? ''
  } catch {
    // Storage can be unavailable in private mode; query attribution still works.
  }
  const merged: AttributionValues = {
    ...existing,
    ...current,
    ...(affiliate ? { aff: affiliate } : {}),
    ...(landingPath ? { landing_path: landingPath } : {}),
    ...(referrer ? { referrer } : {}),
  }

  if (!merged.first_landing_path && landingPath) {
    merged.first_landing_path = landingPath
  }
  if (
    !merged.first_captured_at &&
    (landingPath || Object.keys(current).length > 0)
  ) {
    merged.first_captured_at = new Date().toISOString()
  }
  if (landingPath || referrer || Object.keys(current).length > 0) {
    merged.captured_at = new Date().toISOString()
    merged.expires_at = new Date(Date.now() + ATTRIBUTION_TTL_MS).toISOString()
  }

  const serialized = serializeAttribution(merged)
  if (!serialized) return
  try {
    window.localStorage.setItem(ATTRIBUTION_STORAGE_KEY, serialized)
  } catch {
    // Storage can be unavailable in private mode; registration still works.
  }
}

export function getAdsAttributionPayload(): string {
  captureAttribution()
  const values = readStoredAttribution()
  return serializeAttribution(values)
}
