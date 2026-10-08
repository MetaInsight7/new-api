import i18next from 'i18next'
import { describe, expect, test } from 'vitest'

import { isDashboardSectionAllowed } from '../lib/section-access'
import { getDashboardSectionNavItems } from '../section-registry'

describe('Dashboard Codex limits access', () => {
  test('exposes Codex limits in admin navigation', () => {
    expect(
      getDashboardSectionNavItems(i18next.t, { isAdmin: true })
    ).toContainEqual(
      expect.objectContaining({
        url: '/dashboard/codex-limits',
        title: 'Codex Limits',
      })
    )
    expect(isDashboardSectionAllowed('codex-limits', true)).toBe(true)
  })

  test('hides upstream account limits and user analytics from ordinary users', () => {
    expect(
      getDashboardSectionNavItems(i18next.t).map((item) => item.url)
    ).toEqual(['/dashboard/overview', '/dashboard/models', '/dashboard/flow'])
    expect(isDashboardSectionAllowed('codex-limits')).toBe(false)
    expect(isDashboardSectionAllowed('users')).toBe(false)
    expect(isDashboardSectionAllowed('models')).toBe(true)
  })
})
