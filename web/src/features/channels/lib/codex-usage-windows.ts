export type CodexRateLimitWindow = {
  used_percent?: number
  reset_at?: number
  reset_after_seconds?: number
  limit_window_seconds?: number
}

export type CodexRateLimit = {
  plan_type?: string
  allowed?: boolean
  limit_reached?: boolean
  primary_window?: CodexRateLimitWindow | null
  secondary_window?: CodexRateLimitWindow | null
}

export type CodexRateLimitSource = {
  plan_type?: string
  rate_limit?: CodexRateLimit | null
  primary_window?: CodexRateLimitWindow | null
  secondary_window?: CodexRateLimitWindow | null
}

export type VisibleCodexLimitWindow<
  TWindow extends CodexRateLimitWindow = CodexRateLimitWindow,
> = {
  kind: 'fiveHour' | 'weekly' | 'unknown'
  window: TWindow
}

function hasCodexWindowData(
  window?: CodexRateLimitWindow | null
): window is CodexRateLimitWindow {
  return (
    !!window &&
    [
      window.used_percent,
      window.reset_at,
      window.reset_after_seconds,
      window.limit_window_seconds,
    ].some((value) => typeof value === 'number' && Number.isFinite(value))
  )
}

export function resolveCodexLimitWindows(data: CodexRateLimitSource | null): {
  fiveHourWindow: CodexRateLimitWindow | null
  weeklyWindow: CodexRateLimitWindow | null
  unknownWindows: CodexRateLimitWindow[]
} {
  const primary = data?.rate_limit?.primary_window ?? data?.primary_window
  const secondary = data?.rate_limit?.secondary_window ?? data?.secondary_window
  const planType = (
    data?.plan_type?.trim() ||
    data?.rate_limit?.plan_type?.trim() ||
    ''
  ).toLowerCase()
  let fiveHourWindow: CodexRateLimitWindow | null = null
  let weeklyWindow: CodexRateLimitWindow | null = null
  const unknownWindows: CodexRateLimitWindow[] = []
  for (const window of [primary, secondary]) {
    if (!hasCodexWindowData(window)) continue
    const seconds = window.limit_window_seconds
    if (seconds === 5 * 3600) {
      if (planType === 'free') continue
      if (!fiveHourWindow) {
        fiveHourWindow = window
      } else {
        unknownWindows.push(window)
      }
    } else if (
      seconds != null &&
      seconds >= 6 * 24 * 3600 &&
      seconds <= 8 * 24 * 3600 &&
      !weeklyWindow
    ) {
      weeklyWindow = window
    } else {
      unknownWindows.push(window)
    }
  }
  return { fiveHourWindow, weeklyWindow, unknownWindows }
}

export function getVisibleCodexLimitWindows<
  TWindow extends CodexRateLimitWindow = CodexRateLimitWindow,
>(
  fiveHourWindow?: TWindow | null,
  weeklyWindow?: TWindow | null,
  unknownWindows: TWindow[] = []
): VisibleCodexLimitWindow<TWindow>[] {
  const candidates: Array<{
    kind: VisibleCodexLimitWindow['kind']
    window?: TWindow | null
  }> = [
    { kind: 'fiveHour', window: fiveHourWindow },
    { kind: 'weekly', window: weeklyWindow },
    ...unknownWindows.map((window) => ({ kind: 'unknown' as const, window })),
  ]
  return candidates.filter((item): item is VisibleCodexLimitWindow<TWindow> =>
    hasCodexWindowData(item.window)
  )
}
