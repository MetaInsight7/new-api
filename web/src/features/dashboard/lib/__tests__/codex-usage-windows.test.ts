import { describe, expect, test } from 'vitest'

import {
  getVisibleCodexLimitWindows,
  resolveCodexLimitWindows,
} from '@/features/channels/lib/codex-usage-windows'

describe('Codex upstream windows', () => {
  test('classifies actual durations rather than primary and secondary positions', () => {
    const weekly = { used_percent: 20, limit_window_seconds: 604740 }
    const fiveHour = { used_percent: 0, limit_window_seconds: 18000 }
    expect(
      resolveCodexLimitWindows({
        plan_type: 'pro',
        rate_limit: { primary_window: weekly, secondary_window: fiveHour },
      })
    ).toEqual({
      fiveHourWindow: fiveHour,
      weeklyWindow: weekly,
      unknownWindows: [],
    })
  })

  test('hides genuine five-hour windows for free plans but retains near-week windows', () => {
    const weekly = { used_percent: 60, limit_window_seconds: 604800 }
    expect(
      resolveCodexLimitWindows({
        plan_type: ' FREE ',
        rate_limit: {
          primary_window: { used_percent: 100, limit_window_seconds: 18000 },
          secondary_window: weekly,
        },
      })
    ).toEqual({
      fiveHourWindow: null,
      weeklyWindow: weekly,
      unknownWindows: [],
    })
  })

  test('uses nested plan when root plan is blank and preserves free-plan semantics', () => {
    expect(
      resolveCodexLimitWindows({
        plan_type: ' ',
        rate_limit: {
          plan_type: 'free',
          primary_window: { used_percent: 0, limit_window_seconds: 18000 },
        },
      })
    ).toEqual({ fiveHourWindow: null, weeklyWindow: null, unknownWindows: [] })
  })

  test('keeps unknown and duration-less windows without inventing a weekly window', () => {
    const unknown = { used_percent: 32, limit_window_seconds: 43200 }
    const durationLess = { used_percent: 0 }
    expect(
      resolveCodexLimitWindows({
        primary_window: unknown,
        secondary_window: durationLess,
      })
    ).toEqual({
      fiveHourWindow: null,
      weeklyWindow: null,
      unknownWindows: [unknown, durationLess],
    })
  })

  test('does not turn absent or empty windows into zero usage', () => {
    expect(resolveCodexLimitWindows(null)).toEqual({
      fiveHourWindow: null,
      weeklyWindow: null,
      unknownWindows: [],
    })
    expect(
      getVisibleCodexLimitWindows({}, undefined, [{ used_percent: Number.NaN }])
    ).toEqual([])
    expect(getVisibleCodexLimitWindows(undefined, { used_percent: 0 })).toEqual(
      [{ kind: 'weekly', window: { used_percent: 0 } }]
    )
  })

  test('supports additional-limit root windows when nested windows are absent', () => {
    const fiveHour = { reset_after_seconds: 10, limit_window_seconds: 18000 }
    expect(
      resolveCodexLimitWindows({ rate_limit: {}, primary_window: fiveHour })
    ).toEqual({
      fiveHourWindow: fiveHour,
      weeklyWindow: null,
      unknownWindows: [],
    })
  })
})
