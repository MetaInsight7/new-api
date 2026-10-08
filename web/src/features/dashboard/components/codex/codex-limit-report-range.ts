import { getRollingDateRange } from '@/lib/time'

import type { CodexLimitReportTimeRange } from '../../types'

export function buildCodexLimitReportTimeRange(
  days: number,
  fromDate?: Date
): CodexLimitReportTimeRange {
  const range = getRollingDateRange(days, fromDate)
  return {
    start_timestamp: Math.floor(range.start.getTime() / 1000),
    end_timestamp: Math.floor(range.end.getTime() / 1000),
  }
}

export function isSameCodexLimitReportTimeRange(
  current: CodexLimitReportTimeRange,
  next: CodexLimitReportTimeRange
): boolean {
  return (
    current.start_timestamp === next.start_timestamp &&
    current.end_timestamp === next.end_timestamp
  )
}
