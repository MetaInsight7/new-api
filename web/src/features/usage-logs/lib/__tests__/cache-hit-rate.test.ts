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
*/
import { describe, expect, test } from 'vitest'

import { getCacheHitRate } from '../format'

describe('getCacheHitRate', () => {
  test('uses cached tokens over prompt tokens for OpenAI-style usage', () => {
    expect(
      getCacheHitRate(
        { prompt_tokens: 1000 },
        { cache_tokens: 300, cache_creation_tokens: 0, claude: false }
      )
    ).toBe(30)
  })

  test('includes Claude cache read and creation tokens in the denominator', () => {
    expect(
      getCacheHitRate(
        { prompt_tokens: 1000 },
        { cache_tokens: 300, cache_creation_tokens: 200, claude: true }
      )
    ).toBe(20)
  })

  test('does not report a rate when there are no billable cache tokens', () => {
    expect(
      getCacheHitRate(
        { prompt_tokens: 1000 },
        { cache_tokens: 0, cache_creation_tokens: 200, claude: true }
      )
    ).toBeNull()
  })
})
