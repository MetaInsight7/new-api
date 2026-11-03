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
import { Link, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { TOKEN_DAZI_LOGO } from '@/components/brand/token-dazi-brand-logo'
import { useStatus } from '@/hooks/use-status'

import { AuthLayout } from '../auth-layout'
import { TermsFooter } from '../components/terms-footer'
import { UserAuthForm } from './components/user-auth-form'

export function SignIn() {
  const { t } = useTranslation()
  const { redirect } = useSearch({ from: '/(auth)/sign-in' })
  const { status } = useStatus()

  return (
    <AuthLayout>
      <div className='auth-form-header'>
        <img src={TOKEN_DAZI_LOGO} alt='' className='auth-brand-mark' />
        <div className='auth-heading-copy'>
          <h2 className='auth-title'>{t('Welcome back to Token Dazi')}</h2>
          <p className='auth-subtitle'>
            {t('Models are connected. Turn your ideas into something real.')}
          </p>
        </div>
      </div>

      <UserAuthForm redirectTo={redirect} />

      {!status?.self_use_mode_enabled && status?.register_enabled !== false && (
        <p className='auth-copy-row'>
          {t('New to Token Dazi?')}{' '}
          <Link to='/sign-up'>{t('Sign up now')}</Link>
        </p>
      )}

      <TermsFooter
        variant='sign-in'
        status={status}
        className='mt-4 text-center'
      />
    </AuthLayout>
  )
}
