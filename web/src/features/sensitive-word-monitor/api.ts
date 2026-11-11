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
  SensitiveWordMonitorWindow,
} from './types'

export async function getSensitiveWordMonitorDashboard(
  hours: SensitiveWordMonitorWindow
): Promise<SensitiveWordMonitorDashboard> {
  const response = await api.get<ApiEnvelope<SensitiveWordMonitorDashboard>>(
    '/api/sensitive-words/dashboard',
    { params: { hours } }
  )
  const data = requireServerSuccess(response.data).data
  if (!data || typeof data !== 'object' || Array.isArray(data)) {
    throw new Error('Invalid response from /api/sensitive-words/dashboard')
  }
  return data
}
