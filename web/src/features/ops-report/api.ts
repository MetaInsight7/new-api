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

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'

import type { ApiResponse, OpsReportData } from './types'

export const opsReportQueryKeys = {
  all: ['ops-report'] as const,
  report: (days: number, includeDisabled: boolean, paidOnly: boolean) =>
    [...opsReportQueryKeys.all, days, includeDisabled, paidOnly] as const,
}

export async function getOpsReport(
  days: number,
  includeDisabled: boolean,
  paidOnly: boolean
): Promise<ApiResponse<OpsReportData>> {
  const res = await api.get('/api/data/ops_report', {
    params: {
      days,
      include_disabled: includeDisabled || undefined,
      paid_only: paidOnly || undefined,
    },
  })
  return res.data
}
