import { act, renderHook } from '@testing-library/react'
import i18next from 'i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { OFFICIAL_DOCUMENTATION_URL } from '@/lib/origins'
import { useAuthStore } from '@/stores/auth-store'

import { useTopNavLinks } from '../use-top-nav-links'

const statusFixture = vi.hoisted(() => ({
  status: {} as Record<string, unknown>,
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: statusFixture.status }),
}))

beforeEach(async () => {
  statusFixture.status = {}
  useAuthStore.getState().auth.setUser(null)
  await i18next.changeLanguage('en')
})

afterEach(async () => {
  useAuthStore.getState().auth.setUser(null)
  await i18next.changeLanguage('en')
  i18next.removeResourceBundle('zhCN', 'translation')
})

describe('console website navigation', () => {
  it('defaults to the complete Overlay navigation and opens website pages externally', () => {
    const { result } = renderHook(useTopNavLinks)

    expect(result.current).toEqual([
      { title: 'Home', href: 'https://tokendazi.com/', external: true },
      { title: 'Models', href: 'https://tokendazi.com/models', external: true },
      { title: 'Docs', href: OFFICIAL_DOCUMENTATION_URL, external: true },
      {
        title: 'Rankings',
        href: 'https://tokendazi.com/rankings',
        external: true,
        requiresAuth: false,
      },
    ])
  })

  it('removes a disabled rankings entry without removing core website navigation', () => {
    statusFixture.status = {
      HeaderNavModules: JSON.stringify({
        home: false,
        console: true,
        pricing: false,
        rankings: { enabled: false },
        docs: false,
        about: true,
      }),
    }
    const { result } = renderHook(useTopNavLinks)

    expect(result.current.map((link) => link.title)).toEqual([
      'Home',
      'Models',
      'Docs',
    ])
  })

  it('keeps the rankings sign-in requirement for guests and clears it for signed-in users', () => {
    statusFixture.status = {
      HeaderNavModules: JSON.stringify({
        rankings: { enabled: true, requireAuth: true },
      }),
    }
    const { result, rerender } = renderHook(useTopNavLinks)
    expect(
      result.current.find((link) => link.title === 'Rankings')
    ).toHaveProperty('requiresAuth', true)

    act(() => {
      useAuthStore.getState().auth.setUser({
        id: 1,
        username: 'navigation-user',
        role: 1,
      })
    })
    rerender()

    expect(
      result.current.find((link) => link.title === 'Rankings')
    ).toHaveProperty('requiresAuth', false)
  })

  it('uses safe navigation defaults when backend configuration is malformed', () => {
    statusFixture.status = { HeaderNavModules: '{invalid' }
    const { result } = renderHook(useTopNavLinks)

    expect(result.current.map((link) => link.href)).toEqual([
      'https://tokendazi.com/',
      'https://tokendazi.com/models',
      OFFICIAL_DOCUMENTATION_URL,
      'https://tokendazi.com/rankings',
    ])
  })

  it('updates translated labels without localizing external website URLs', async () => {
    i18next.addResourceBundle('zhCN', 'translation', {
      Home: '主页',
      Models: '模型',
      Docs: '文档',
      Rankings: '排行榜',
    })
    const { result } = renderHook(useTopNavLinks)
    const websiteUrls = result.current.map((link) => link.href)

    await act(() => i18next.changeLanguage('zhCN').then(() => undefined))

    expect(result.current.map((link) => link.title)).toEqual([
      '主页',
      '模型',
      '文档',
      '排行榜',
    ])
    expect(result.current.map((link) => link.href)).toEqual(websiteUrls)
  })
})
