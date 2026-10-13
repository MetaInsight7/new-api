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
import type { ChannelConcurrencyStatus } from '@/features/channels/api'
import type { Channel } from '@/features/channels/types'

export type ChannelLoadState = 'all' | 'full' | 'queued' | 'cooling'

export type ChannelLoadSortBy = 'priority' | 'id' | 'load' | 'waiting'
export type ChannelLoadSortOrder = 'asc' | 'desc'

export type ChannelLoadMonitorParams = {
  p: number
  page_size: number
  keyword?: string
  status?: string
  type?: number
  group?: string
  load_state?: ChannelLoadState
  sort_by?: ChannelLoadSortBy
  sort_order?: ChannelLoadSortOrder
}

export type ChannelLoadMonitorItem = {
  channel: Channel
  concurrency: ChannelConcurrencyStatus | null
}

export type ChannelLoadMonitorSummary = {
  limited_rpm: number
  active: number
  capacity: number
  waiting: number
  cooling_down: number
}

export type ChannelLoadMonitorData = {
  items: ChannelLoadMonitorItem[]
  total: number
  page: number
  page_size: number
  type_counts: Record<string, number>
  groups: string[]
  summary: ChannelLoadMonitorSummary
  snapshot_at: number
}
