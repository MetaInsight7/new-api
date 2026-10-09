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
*/
const DEFAULT_OFFICIAL_DOCUMENTATION_URL = 'https://docs.tokendazi.com/'

export function normalizeOrigin(origin: string | undefined): string {
  return origin?.trim().replace(/\/+$/, '') ?? ''
}

export function resolveOfficialWebsiteOrigin(
  configuredOrigin: string | undefined
): string {
  return normalizeOrigin(configuredOrigin) || 'https://tokendazi.com'
}

export const OFFICIAL_WEBSITE_ORIGIN = resolveOfficialWebsiteOrigin(
  import.meta.env.VITE_OFFICIAL_WEBSITE_ORIGIN as string | undefined
)

export function officialWebsiteUrl(
  path: string,
  origin = OFFICIAL_WEBSITE_ORIGIN
): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  return origin ? `${normalizeOrigin(origin)}${normalizedPath}` : normalizedPath
}

export function resolveOfficialDocumentationUrl(
  configuredUrl: string | undefined
): string {
  return configuredUrl?.trim() || DEFAULT_OFFICIAL_DOCUMENTATION_URL
}

export const OFFICIAL_DOCUMENTATION_URL = resolveOfficialDocumentationUrl(
  import.meta.env.VITE_OFFICIAL_DOCUMENTATION_URL as string | undefined
)
