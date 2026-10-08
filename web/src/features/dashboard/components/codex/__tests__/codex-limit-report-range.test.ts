import { describe, expect, test } from 'vitest'

import {
  buildCodexLimitReportTimeRange,
  isSameCodexLimitReportTimeRange,
} from '../codex-limit-report-range'

describe('Codex report rolling range', () => {
  test.each([1, 7, 14, 29])(
    'uses exactly %i rolling days ending at the requested instant',
    (days) => {
      const range = buildCodexLimitReportTimeRange(
        days,
        new Date('2026-10-07T10:15:30.999Z')
      )
      expect(range.end_timestamp).toBe(1791368130)
      expect(range.end_timestamp - range.start_timestamp).toBe(days * 86400)
    }
  )

  test('compares both boundaries so refresh can advance the reporting interval', () => {
    const current = { start_timestamp: 100, end_timestamp: 200 }
    expect(isSameCodexLimitReportTimeRange(current, { ...current })).toBe(true)
    expect(
      isSameCodexLimitReportTimeRange(current, {
        start_timestamp: 101,
        end_timestamp: 200,
      })
    ).toBe(false)
    expect(
      isSameCodexLimitReportTimeRange(current, {
        start_timestamp: 100,
        end_timestamp: 201,
      })
    ).toBe(false)
  })
})
