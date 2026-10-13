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
import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'

import { ChannelLoadMonitor } from '@/features/channel-load-monitor'
import { hasPermission } from '@/lib/admin-permissions'
import { ROLE } from '@/lib/roles'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

export const channelLoadMonitorSearchSchema = z.object({
  page: z.number().int().positive().optional().catch(1),
  pageSize: z.number().int().positive().max(100).optional().catch(undefined),
  filter: z.string().optional().catch(''),
  status: z
    .enum(['all', 'enabled', 'disabled', 'banned'])
    .optional()
    .catch('all'),
  type: z.string().optional().catch('all'),
  group: z.string().optional().catch('all'),
  loadState: z
    .enum(['all', 'full', 'queued', 'cooling'])
    .optional()
    .catch('all'),
  sortBy: z
    .enum(['priority', 'id', 'load', 'waiting'])
    .optional()
    .catch('priority'),
  sortOrder: z.enum(['asc', 'desc']).optional().catch('desc'),
})

export function canAccessChannelLoadMonitor(
  user: AuthUser | null | undefined
): boolean {
  return Boolean(
    user && user.role >= ROLE.ADMIN && hasPermission(user, 'channel', 'read')
  )
}

export const Route = createFileRoute('/_authenticated/channel-load-monitor/')({
  beforeLoad: () => {
    const user = useAuthStore.getState().auth.user
    if (!canAccessChannelLoadMonitor(user)) {
      throw redirect({ to: '/403' })
    }
  },
  validateSearch: channelLoadMonitorSearchSchema,
  component: ChannelLoadMonitor,
})
