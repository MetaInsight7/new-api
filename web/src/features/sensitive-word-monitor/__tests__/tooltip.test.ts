/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { describe, expect, it } from 'vitest'

import { getSensitiveWordTooltipTimestamp } from '../lib'

describe('sensitive-word tooltip timestamp extraction', () => {
  it('reads timestamps from chart payloads and rejects missing or invalid values', () => {
    expect(
      getSensitiveWordTooltipTimestamp([
        {
          payload: {
            timestamp: 1_800_000_000,
          },
        },
      ])
    ).toBe(1_800_000_000)
    expect(getSensitiveWordTooltipTimestamp(undefined)).toBeNull()
    expect(
      getSensitiveWordTooltipTimestamp([
        {
          payload: {
            timestamp: 'not-a-timestamp',
          },
        },
      ])
    ).toBeNull()
  })
})
