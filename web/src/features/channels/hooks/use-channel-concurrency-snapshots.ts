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
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'

import {
  getChannelConcurrencyStatus,
  type ChannelConcurrencyStatus,
} from '../api'
import { channelsQueryKeys, isTagAggregateRow } from '../lib'
import type { Channel } from '../types'

export const CHANNEL_CONCURRENCY_REFRESH_INTERVAL = 10_000

export function useChannelConcurrencySnapshots(channels: readonly Channel[]) {
  const channelIds = useMemo(() => {
    const ids = new Set<number>()
    const pending = [...channels]
    while (pending.length > 0) {
      const channel = pending.pop()
      if (!channel) continue
      if (isTagAggregateRow(channel)) {
        pending.push(...channel.children)
        continue
      }
      if (channel.id > 0 && channel.max_concurrency > 0) {
        ids.add(channel.id)
      }
    }
    return [...ids].sort((left, right) => left - right)
  }, [channels])

  const query = useQuery({
    queryKey: channelsQueryKeys.concurrency(channelIds),
    queryFn: ({ signal }) => getChannelConcurrencyStatus(channelIds, signal),
    enabled: channelIds.length > 0,
    staleTime: 0,
    refetchOnMount: 'always',
    refetchOnWindowFocus: false,
    refetchInterval: CHANNEL_CONCURRENCY_REFRESH_INTERVAL,
    refetchIntervalInBackground: false,
    retry: false,
    meta: { errorToast: false },
  })

  const statusById = useMemo(() => {
    const statuses = new Map<number, ChannelConcurrencyStatus>()
    for (const status of query.data ?? []) {
      statuses.set(status.channel_id, status)
    }
    return statuses
  }, [query.data])

  return { ...query, channelIds, statusById }
}
