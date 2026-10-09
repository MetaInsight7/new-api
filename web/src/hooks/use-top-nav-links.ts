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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { useStatus } from '@/hooks/use-status'
import {
  type HeaderNavModules,
  parseHeaderNavModulesFromStatus,
} from '@/lib/nav-modules'
import { OFFICIAL_DOCUMENTATION_URL, officialWebsiteUrl } from '@/lib/origins'
import { useAuthStore } from '@/stores/auth-store'

export type TopNavLink = {
  title: string
  href: string
  disabled?: boolean
  requiresAuth?: boolean
  external?: boolean
}

type BuildTopNavLinksOptions = {
  translate: (key: string) => string
  modules: HeaderNavModules
  isAuthed: boolean
}

export function buildTopNavLinks(
  options: BuildTopNavLinksOptions
): TopNavLink[] {
  const websiteLink = (title: string, path: string): TopNavLink => {
    const href = officialWebsiteUrl(path)
    return { title, href, external: href.startsWith('http') }
  }

  const links: TopNavLink[] = [
    websiteLink(options.translate('Home'), '/'),
    websiteLink(options.translate('Models'), '/models'),
  ]

  if (OFFICIAL_DOCUMENTATION_URL) {
    links.push({
      title: options.translate('Docs'),
      href: OFFICIAL_DOCUMENTATION_URL,
      external: true,
    })
  }

  const rankings = options.modules.rankings
  if (rankings.enabled) {
    links.push({
      ...websiteLink(options.translate('Rankings'), '/rankings'),
      requiresAuth: rankings.requireAuth && !options.isAuthed,
    })
  }

  return links
}

export function useTopNavLinks(): TopNavLink[] {
  const { t } = useTranslation()
  const { status } = useStatus()
  const { auth } = useAuthStore()

  // Parse HeaderNavModules
  const modules = useMemo(() => {
    return parseHeaderNavModulesFromStatus(
      status as Record<string, unknown> | null
    )
  }, [status])

  return buildTopNavLinks({
    translate: t,
    modules,
    isAuthed: !!auth?.user,
  })
}
