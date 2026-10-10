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
import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import {
  ADMIN_PERMISSION_ACTIONS,
  ADMIN_PERMISSION_RESOURCES,
  hasPermission,
} from '@/lib/admin-permissions'
import { useAuthStore } from '@/stores/auth-store'

import type { ChannelConcurrencyStatus } from '../api'
import {
  createChannelFieldUpdateScheduler,
  handleUpdateChannelMaxConcurrency,
  isTagAggregateRow,
} from '../lib'
import type { Channel } from '../types'
import { NumericSpinnerInput } from './numeric-spinner-input'

type ConcurrencyState = 'idle' | 'busy' | 'saturated'

const concurrencyVariants: Record<ConcurrencyState, StatusVariant> = {
  idle: 'neutral',
  busy: 'warning',
  saturated: 'danger',
}

export function ChannelConcurrencyStatusDisplay({
  status,
}: {
  status?: ChannelConcurrencyStatus
}) {
  const { t } = useTranslation()
  if (!status || status.max_concurrency <= 0) {
    return (
      <span
        className='text-muted-foreground text-xs'
        aria-label={t('Channel concurrency unavailable')}
      >
        -
      </span>
    )
  }

  let state: ConcurrencyState = 'busy'
  if (status.active <= 0) {
    state = 'idle'
  } else if (status.active >= status.max_concurrency) {
    state = 'saturated'
  }

  const accessibleParts = [
    t('Channel concurrency: {{active}} of {{max}} active', {
      active: status.active,
      max: status.max_concurrency,
    }),
  ]
  if (status.waiting > 0) {
    accessibleParts.push(t('{{count}} waiting', { count: status.waiting }))
  }
  if (status.cooling_down) {
    accessibleParts.push(t('Cooling'))
  }

  return (
    <span
      className='flex min-w-0 flex-wrap items-center gap-1'
      aria-label={accessibleParts.join(', ')}
      data-concurrency-state={state}
    >
      <StatusBadge
        label={`${status.active}/${status.max_concurrency}`}
        variant={concurrencyVariants[state]}
        size='sm'
        copyable={false}
        aria-hidden='true'
      />
      {status.waiting > 0 && (
        <StatusBadge
          label={`+${status.waiting}`}
          variant='neutral'
          size='sm'
          copyable={false}
          aria-hidden='true'
        />
      )}
      {status.cooling_down && (
        <StatusBadge
          label={t('Cooling')}
          variant='blue'
          size='sm'
          copyable={false}
          aria-hidden='true'
        />
      )}
    </span>
  )
}

export function ChannelConcurrencyCell({
  channel,
  status,
}: {
  channel: Channel
  status?: ChannelConcurrencyStatus
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const currentUser = useAuthStore((state) => state.auth.user)
  const isTagRow = isTagAggregateRow(channel)
  const canEdit =
    !isTagRow &&
    hasPermission(
      currentUser,
      ADMIN_PERMISSION_RESOURCES.CHANNEL,
      ADMIN_PERMISSION_ACTIONS.WRITE
    )
  const visibleStatus =
    !isTagRow && channel.max_concurrency > 0 ? status : undefined
  const [draftValue, setDraftValue] = useState(channel.max_concurrency)
  const [isSaving, setIsSaving] = useState(false)

  const saveValue = useCallback(
    async (nextValue: number) => {
      if (nextValue === channel.max_concurrency) return
      setIsSaving(true)
      const saved = await handleUpdateChannelMaxConcurrency(
        channel.id,
        nextValue,
        queryClient
      )
      if (!saved) {
        setDraftValue(channel.max_concurrency)
      }
      setIsSaving(false)
    },
    [channel.id, channel.max_concurrency, queryClient]
  )
  const updateScheduler = useMemo(
    () =>
      createChannelFieldUpdateScheduler((nextValue) => {
        void saveValue(nextValue)
      }),
    [saveValue]
  )

  useEffect(() => {
    setDraftValue(channel.max_concurrency)
  }, [channel.max_concurrency])

  useEffect(() => () => updateScheduler.flush(), [updateScheduler])

  if (!canEdit) {
    return <ChannelConcurrencyStatusDisplay status={visibleStatus} />
  }

  return (
    <span className='flex min-w-0 flex-wrap items-center gap-2'>
      <ChannelConcurrencyStatusDisplay status={visibleStatus} />
      <span role='group' aria-label={t('Maximum Concurrency')}>
        <NumericSpinnerInput
          value={draftValue}
          onChange={(nextValue) => {
            setDraftValue(nextValue)
            updateScheduler.schedule(nextValue)
          }}
          onCommit={updateScheduler.flush}
          min={0}
          step={1}
          disabled={isSaving}
          rejectInvalid
          onInvalid={() =>
            toast.error(
              t('Maximum concurrency must be a non-negative integer.')
            )
          }
        />
      </span>
    </span>
  )
}
