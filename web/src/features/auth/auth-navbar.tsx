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
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { TokenDaziBrandLogo } from '@/components/brand/token-dazi-brand-logo'

import './auth.css'

export function AuthNavbar() {
  const { t } = useTranslation()
  const [navigationOpen, setNavigationOpen] = useState(false)
  const menuButtonRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    const mobileBreakpoint = window.matchMedia('(max-width: 760px)')
    const closeNavigation = () => setNavigationOpen(false)
    mobileBreakpoint.addEventListener('change', closeNavigation)
    return () => mobileBreakpoint.removeEventListener('change', closeNavigation)
  }, [])

  useEffect(() => {
    if (!navigationOpen) return

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      setNavigationOpen(false)
      menuButtonRef.current?.focus()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [navigationOpen])

  const closeNavigation = () => setNavigationOpen(false)

  return (
    <header className='auth-navbar'>
      <nav className='auth-navbar-inner' aria-label={t('Main navigation')}>
        <a
          href='https://tokendazi.com/'
          className='auth-navbar-brand'
          aria-label={t('Token Dazi home')}
        >
          <TokenDaziBrandLogo alt='' />
        </a>
        <div
          id='auth-navbar-links'
          className={`auth-navbar-links${navigationOpen ? ' open' : ''}`}
        >
          <a href='https://tokendazi.com/' onClick={closeNavigation}>
            {t('Home')}
          </a>
          <a href='https://wiki.tokendazi.com/cn' onClick={closeNavigation}>
            {t('Quick start')}
          </a>
          <a href='https://tokendazi.com/models' onClick={closeNavigation}>
            {t('Models')}
          </a>
          <a href='https://tokendazi.com/rankings' onClick={closeNavigation}>
            {t('Rankings')}
          </a>
        </div>
        <div className='auth-navbar-actions'>
          <a href='/sign-in'>{t('Login')}</a>
          <a className='auth-navbar-action-primary' href='/sign-up'>
            {t('Free registration')}
          </a>
        </div>
        <button
          ref={menuButtonRef}
          className='auth-navbar-menu'
          type='button'
          aria-label={t(
            navigationOpen ? 'Close navigation' : 'Open navigation'
          )}
          aria-controls='auth-navbar-links'
          aria-expanded={navigationOpen}
          onClick={() => setNavigationOpen((open) => !open)}
        >
          <span aria-hidden='true'>☰</span>
        </button>
      </nav>
    </header>
  )
}
