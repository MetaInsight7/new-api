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
*/
import { describe, expect, test } from 'vitest'

import { hasTimeDependentBillingExpression } from '../use-billing-time'

describe('billing time expression detection', () => {
  test('detects time functions in provider expression collections', () => {
    expect(
      hasTimeDependentBillingExpression([
        'tier("default", p * 1 + c * 4)',
        'hour("Asia/Shanghai") < 12 ? tier("am", p * 2 + c * 8) : tier("pm", p * 3 + c * 12)',
      ])
    ).toBe(true)
  })

  test('ignores empty, invalid, and non-time expressions', () => {
    expect(
      hasTimeDependentBillingExpression([
        undefined,
        '',
        'not valid billing syntax',
        'tier("default", p * 1 + c * 4)',
      ])
    ).toBe(false)
  })
})
