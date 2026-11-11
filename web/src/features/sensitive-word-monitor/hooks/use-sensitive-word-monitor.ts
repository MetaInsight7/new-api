/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { useQuery, useQueryClient } from '@tanstack/react-query'

import { getSensitiveWordMonitorDashboard } from '../api'
import type { SensitiveWordMonitorWindow } from '../types'

export const sensitiveWordMonitorQueryKeys = {
  all: ['sensitive-word-monitor'] as const,
  dashboard: (hours: SensitiveWordMonitorWindow) =>
    [...sensitiveWordMonitorQueryKeys.all, 'dashboard', hours] as const,
}

export function useSensitiveWordMonitorDashboard(
  hours: SensitiveWordMonitorWindow
) {
  return useQuery({
    queryKey: sensitiveWordMonitorQueryKeys.dashboard(hours),
    queryFn: () => getSensitiveWordMonitorDashboard(hours),
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
