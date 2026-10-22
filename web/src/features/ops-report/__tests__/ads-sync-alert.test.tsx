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
import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import { AdsSyncAlert } from '../ads-sync-alert'

test('shows a sanitized warning while preserving the last successful data', () => {
  render(<AdsSyncAlert error='ads spend synchronization failed' />)

  expect(screen.getByRole('alert')).toHaveTextContent(
    'Google Ads spend synchronization failed'
  )
  expect(screen.getByRole('alert')).toHaveTextContent(
    'Showing the last successfully synced data.'
  )
})

test('renders nothing when the latest sync is healthy', () => {
  const { container } = render(<AdsSyncAlert error='' />)

  expect(container).toBeEmptyDOMElement()
})
