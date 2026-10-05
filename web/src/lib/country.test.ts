/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, test } from 'vitest'

import { countryLabel } from './country'

describe('countryLabel', () => {
  test('renders a flag and localized country name', () => {
    expect(countryLabel('US', 'zhCN')).toBe('🇺🇸 美国')
    expect(countryLabel('sg', 'en-US')).toBe('🇸🇬 Singapore')
  })

  test('returns an empty label for invalid country codes', () => {
    expect(countryLabel('', 'zhCN')).toBe('')
    expect(countryLabel('USA', 'zhCN')).toBe('')
    expect(countryLabel('::1', 'zhCN')).toBe('')
  })
})
