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
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { AuthLayout } from '../auth-layout'

type AuthWindow = Window & {
  __newApiAuthPointer?: {
    x: number
    y: number
  }
}

afterEach(() => {
  window.sessionStorage.clear()
  delete (window as AuthWindow).__newApiAuthPointer
  vi.restoreAllMocks()
})

describe('authentication layout', () => {
  test('matches the committed public-site navigation and keeps the form in its own region', () => {
    const { container } = render(
      <AuthLayout>
        <form aria-label='Authentication form'>Form content</form>
      </AuthLayout>
    )

    expect(screen.getByRole('banner')).toBeInTheDocument()
    expect(
      screen.getByRole('navigation', { name: 'Main navigation' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: 'Token Dazi home' })
    ).toHaveAttribute('href', 'https://tokendazi.com/')
    expect(container.querySelector('.auth-navbar-brand img')).toHaveAttribute(
      'src',
      '/token-dazi-logo.svg'
    )
    expect(screen.getByRole('link', { name: 'Home' })).toHaveAttribute(
      'href',
      'https://tokendazi.com/'
    )
    expect(screen.getByRole('link', { name: 'Quick start' })).toHaveAttribute(
      'href',
      'https://wiki.tokendazi.com/cn'
    )
    expect(screen.getByRole('link', { name: 'Models' })).toHaveAttribute(
      'href',
      'https://tokendazi.com/models'
    )
    expect(screen.getByRole('link', { name: 'Rankings' })).toHaveAttribute(
      'href',
      'https://tokendazi.com/rankings'
    )
    expect(screen.getByRole('link', { name: 'Login' })).toHaveAttribute(
      'href',
      '/sign-in'
    )
    expect(
      screen.getByRole('link', { name: 'Free registration' })
    ).toHaveAttribute('href', '/sign-up')
    expect(container.querySelector('.auth-visual-panel')).toHaveAttribute(
      'aria-hidden',
      'true'
    )
    expect(
      screen
        .getByRole('form', { name: 'Authentication form' })
        .closest('.auth-form-shell')
    ).not.toBeNull()
  })

  test('opens the mobile navigation and closes it with Escape while restoring focus', () => {
    render(<AuthLayout>Form content</AuthLayout>)
    const menuButton = screen.getByRole('button', {
      name: 'Open navigation',
    })

    expect(menuButton).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(menuButton)

    expect(
      screen.getByRole('button', { name: 'Close navigation' })
    ).toHaveAttribute('aria-expanded', 'true')

    fireEvent.keyDown(document, { key: 'Escape' })

    expect(menuButton).toHaveAttribute('aria-expanded', 'false')
    expect(menuButton).toHaveAccessibleName('Open navigation')
    expect(menuButton).toHaveFocus()
  })

  test('closes the mobile navigation when the responsive breakpoint changes', () => {
    let breakpointListener: EventListener | undefined
    vi.spyOn(window, 'matchMedia').mockImplementation(
      (query: string): MediaQueryList => ({
        matches:
          query.includes('prefers-reduced-motion') &&
          !query.includes('no-preference'),
        media: query,
        onchange: null,
        addListener: () => undefined,
        removeListener: () => undefined,
        addEventListener: (
          type: string,
          listener: EventListenerOrEventListenerObject
        ) => {
          if (query === '(max-width: 760px)' && type === 'change') {
            breakpointListener = listener as EventListener
          }
        },
        removeEventListener: () => undefined,
        dispatchEvent: () => false,
      })
    )
    render(<AuthLayout>Form content</AuthLayout>)
    const menuButton = screen.getByRole('button', {
      name: 'Open navigation',
    })
    fireEvent.click(menuButton)

    expect(menuButton).toHaveAttribute('aria-expanded', 'true')
    act(() => breakpointListener?.(new Event('change')))

    expect(menuButton).toHaveAttribute('aria-expanded', 'false')
    expect(menuButton).toHaveAccessibleName('Open navigation')
  })

  test('moves mascot eyes toward the pointer when motion is enabled', async () => {
    vi.spyOn(window, 'matchMedia').mockImplementation(
      (query: string): MediaQueryList => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: () => undefined,
        removeListener: () => undefined,
        addEventListener: () => undefined,
        removeEventListener: () => undefined,
        dispatchEvent: () => false,
      })
    )
    const { container } = render(<AuthLayout>Form content</AuthLayout>)
    const pupil = container.querySelector<HTMLElement>('.auth-pupil')

    expect(pupil).not.toBeNull()
    await waitFor(() =>
      expect(pupil).toHaveStyle({ transform: 'translate(5px, 0px)' })
    )

    fireEvent.mouseMove(window, { clientX: 0, clientY: 500 })

    await waitFor(() =>
      expect(pupil?.style.transform).not.toBe('translate(5px, 0px)')
    )
    expect(window.sessionStorage.getItem('new-api-auth-pointer')).toBe(
      '{"x":0,"y":500}'
    )
  })

  test('leaves mascot eyes still when reduced motion is preferred', () => {
    const { container } = render(<AuthLayout>Form content</AuthLayout>)
    const pupil = container.querySelector<HTMLElement>('.auth-pupil')
    const initialTransform = pupil?.style.transform

    expect(initialTransform).toBeTruthy()
    fireEvent.mouseMove(window, { clientX: 0, clientY: 500 })
    expect(pupil?.style.transform).toBe(initialTransform)
  })
})
