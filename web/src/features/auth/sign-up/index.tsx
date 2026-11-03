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
import { Link } from '@tanstack/react-router'
import { CheckCircle2, KeyRound, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { TOKEN_DAZI_LOGO } from '@/components/brand/token-dazi-brand-logo'
import { useStatus } from '@/hooks/use-status'

import { AuthLayout } from '../auth-layout'
import { TermsFooter } from '../components/terms-footer'
import { SignUpForm } from './components/sign-up-form'

export function SignUp() {
  const { t } = useTranslation()
  const { status } = useStatus()

  return (
    <AuthLayout compact>
      <div className='auth-form-header'>
        <img src={TOKEN_DAZI_LOGO} alt='' className='auth-brand-mark' />
        <div className='auth-heading-copy'>
          <h2 className='auth-title'>{t('Your copilot, always online')}</h2>
          <p className='auth-subtitle'>
            {t('One key, countless models. Ideas spark, code follows.')}
          </p>
        </div>
      </div>

      <div className='auth-register-steps'>
        <div className='auth-register-step'>
          <span className='auth-register-step-icon auth-step-account'>
            <KeyRound className='h-4 w-4' aria-hidden='true' />
          </span>
          <span className='auth-register-step-label'>{t('Create access')}</span>
        </div>
        <span className='auth-register-step-line' aria-hidden='true' />
        <div className='auth-register-step'>
          <span className='auth-register-step-icon auth-step-verify'>
            <CheckCircle2 className='h-4 w-4' aria-hidden='true' />
          </span>
          <span className='auth-register-step-label'>
            {t('Connect models')}
          </span>
        </div>
        <span className='auth-register-step-line' aria-hidden='true' />
        <div className='auth-register-step'>
          <span className='auth-register-step-icon auth-step-ready'>
            <Sparkles className='h-4 w-4' aria-hidden='true' />
          </span>
          <span className='auth-register-step-label'>
            {t('Start creating')}
          </span>
        </div>
      </div>

      <SignUpForm />

      <p className='auth-copy-row'>
        {t('Already have Token Dazi?')}{' '}
        <Link to='/sign-in'>{t('Sign in now')}</Link>
      </p>

      <TermsFooter
        variant='sign-up'
        status={status}
        className='mt-4 text-center'
      />
    </AuthLayout>
  )
}
