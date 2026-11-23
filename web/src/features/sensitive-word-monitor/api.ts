/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import type {
  ApiEnvelope,
  SensitiveWordMonitorDashboard,
  SensitiveWordMonitorEventFilters,
  SensitiveWordMonitorEventPage,
  SensitiveWordMonitorRange,
} from './types'

function sensitiveWordMonitorRangeParams(range: SensitiveWordMonitorRange) {
  if (range.kind === 'preset') return { hours: range.hours }
  return {
    start_timestamp: range.start_timestamp,
    end_timestamp: range.end_timestamp,
  }
}

export async function getSensitiveWordMonitorDashboard(
  range: SensitiveWordMonitorRange
): Promise<SensitiveWordMonitorDashboard> {
  const response = await api.get<ApiEnvelope<SensitiveWordMonitorDashboard>>(
    '/api/sensitive-words/dashboard',
    { params: sensitiveWordMonitorRangeParams(range) }
  )
  const data = requireServerSuccess(response.data).data
  if (!data || typeof data !== 'object' || Array.isArray(data)) {
    throw new Error('Invalid response from /api/sensitive-words/dashboard')
  }
  return data
}

export async function getSensitiveWordMonitorEvents(
  range: SensitiveWordMonitorRange,
  filters: SensitiveWordMonitorEventFilters
): Promise<SensitiveWordMonitorEventPage> {
  const response = await api.get<ApiEnvelope<SensitiveWordMonitorEventPage>>(
    '/api/sensitive-words/events',
    {
      params: {
        ...sensitiveWordMonitorRangeParams(range),
        p: filters.page,
        page_size: filters.pageSize,
        disposition: filters.disposition || undefined,
        keyword: filters.keyword || undefined,
      },
    }
  )
  const data = requireServerSuccess(response.data).data
  if (
    !data ||
    typeof data !== 'object' ||
    Array.isArray(data) ||
    !Array.isArray(data.items)
  ) {
    throw new Error('Invalid response from /api/sensitive-words/events')
  }
  return data
}
