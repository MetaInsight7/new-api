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
import { getUserQuotaDates } from '@/features/dashboard/api'
import {
  buildQueryParams,
  calculateDashboardStats,
  getDefaultDays,
} from '@/features/dashboard/lib'
import type { DashboardFilters } from '@/features/dashboard/types'

export async function getWorkspaceUsage(filters: DashboardFilters = {}) {
  const days = getDefaultDays(filters.time_granularity)
  const now = Math.floor(Date.now() / 1000)
  const response = await getUserQuotaDates(
    buildQueryParams(
      {
        start_timestamp: filters.start_timestamp
          ? Math.floor(filters.start_timestamp.getTime() / 1000)
          : now - days * 86400,
        end_timestamp: filters.end_timestamp
          ? Math.floor(filters.end_timestamp.getTime() / 1000)
          : now,
      },
      filters
    ),
    false
  )
  const data = response?.data ?? []
  const stats = calculateDashboardStats(data)
  return { data, ...stats }
}
