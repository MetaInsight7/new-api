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
import { fireEvent, render, screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { LegalConsent } from '../legal-consent'

test('uses the branded consent control and keeps legal documents secure', () => {
  const onCheckedChange = vi.fn()

  render(
    <div className='auth-page'>
      <LegalConsent
        status={{
          user_agreement_enabled: true,
          privacy_policy_enabled: true,
        }}
        checked={false}
        onCheckedChange={onCheckedChange}
      />
    </div>
  )

  const checkbox = screen.getByRole('checkbox')
  expect(checkbox.closest('.auth-legal-consent')).not.toBeNull()
  expect(screen.getByRole('link', { name: 'User Agreement' })).toHaveAttribute(
    'rel',
    'noopener noreferrer'
  )
  expect(screen.getByRole('link', { name: 'Privacy Policy' })).toHaveAttribute(
    'target',
    '_blank'
  )

  fireEvent.click(checkbox)
  expect(onCheckedChange).toHaveBeenCalledWith(true)
})
