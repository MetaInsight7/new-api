/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import {
  keepPreviousData,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'

import {
  getSensitiveWordMonitorDashboard,
  getSensitiveWordMonitorEvents,
} from '../api'
import type {
  SensitiveWordMonitorEventFilters,
  SensitiveWordMonitorRange,
} from '../types'

function rangeKey(range: SensitiveWordMonitorRange) {
  return range.kind === 'preset'
    ? ['preset', range.hours]
    : ['custom', range.start_timestamp, range.end_timestamp]
}

export const sensitiveWordMonitorQueryKeys = {
  all: ['sensitive-word-monitor'] as const,
  dashboard: (range: SensitiveWordMonitorRange) =>
    [
      ...sensitiveWordMonitorQueryKeys.all,
      'dashboard',
      ...rangeKey(range),
    ] as const,
  events: (
    range: SensitiveWordMonitorRange,
    filters: SensitiveWordMonitorEventFilters
  ) =>
    [
      ...sensitiveWordMonitorQueryKeys.all,
      'events',
      ...rangeKey(range),
      filters.page,
      filters.pageSize,
      filters.disposition,
      filters.keyword,
    ] as const,
}

export function useSensitiveWordMonitorDashboard(
  range: SensitiveWordMonitorRange
) {
  return useQuery({
    queryKey: sensitiveWordMonitorQueryKeys.dashboard(range),
    queryFn: () => getSensitiveWordMonitorDashboard(range),
    staleTime: 30 * 1000,
    retry: false,
  })
}

export function useSensitiveWordMonitorEvents(
  range: SensitiveWordMonitorRange,
  filters: SensitiveWordMonitorEventFilters
) {
  return useQuery({
    queryKey: sensitiveWordMonitorQueryKeys.events(range, filters),
    queryFn: () => getSensitiveWordMonitorEvents(range, filters),
    placeholderData: keepPreviousData,
    staleTime: 30 * 1000,
    retry: false,
  })
}

export function useRefreshSensitiveWordMonitor(): () => Promise<void> {
  const queryClient = useQueryClient()
  return async () => {
    await queryClient.invalidateQueries({
      queryKey: sensitiveWordMonitorQueryKeys.all,
    })
  }
}
