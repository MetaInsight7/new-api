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
import type { TFunction } from 'i18next'
import { LoaderCircle } from 'lucide-react'
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  ADMIN_PERMISSION_ACTIONS,
  ADMIN_PERMISSION_RESOURCES,
  hasPermission,
} from '@/lib/admin-permissions'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import type { ChannelConcurrencyStatus } from '../api'
import {
  createChannelFieldUpdateScheduler,
  getChannelConcurrencyState,
  handleUpdateChannelMaxConcurrency,
  isTagAggregateRow,
  type ChannelConcurrencyState,
} from '../lib'
import type { Channel } from '../types'
import { NumericSpinnerInput } from './numeric-spinner-input'

const concurrencyVariants: Record<ChannelConcurrencyState, StatusVariant> = {
  unlimited: 'neutral',
  unavailable: 'neutral',
  idle: 'neutral',
  busy: 'warning',
  saturated: 'danger',
  over_limit: 'danger',
}

function channelConcurrencyAccessibleLabel(
  t: TFunction,
  maxConcurrency: number,
  status?: ChannelConcurrencyStatus
) {
  if (maxConcurrency <= 0) return t('Unlimited concurrency')
  if (!status) return t('Channel concurrency unavailable')

  const parts = [
    t('Channel concurrency: {{active}} of {{max}} active', {
      active: status.active,
      max: maxConcurrency,
    }),
  ]
  if (status.active > maxConcurrency) parts.push(t('Over limit'))
  if (status.waiting > 0) {
    parts.push(t('{{count}} waiting', { count: status.waiting }))
  }
  if (status.cooling_down) parts.push(t('Cooling'))
  return parts.join(', ')
}

function ChannelConcurrencyBadges({
  maxConcurrency,
  status,
  maxTrigger,
  showSecondaryBadges = true,
}: {
  maxConcurrency: number
  status?: ChannelConcurrencyStatus
  maxTrigger?: ReactNode
  showSecondaryBadges?: boolean
}) {
  const { t } = useTranslation()
  const state = getChannelConcurrencyState(maxConcurrency, status)
  const active = status?.active
  const primary =
    maxConcurrency <= 0 ? (
      (maxTrigger ?? t('Unlimited'))
    ) : (
      <>
        <span>{active === undefined ? '-' : active}/</span>
        {maxTrigger ?? <span>{maxConcurrency}</span>}
      </>
    )

  return (
    <span
      className='flex min-w-0 flex-nowrap items-center gap-1'
      aria-label={channelConcurrencyAccessibleLabel(t, maxConcurrency, status)}
      data-concurrency-state={state}
    >
      <StatusBadge
        variant={concurrencyVariants[state]}
        size='sm'
        copyable={false}
        aria-hidden={maxTrigger ? undefined : true}
      >
        {primary}
      </StatusBadge>
      {showSecondaryBadges && status && status.active > maxConcurrency && (
        <StatusBadge
          label={t('Over limit')}
          variant='danger'
          size='sm'
          copyable={false}
          aria-hidden='true'
        />
      )}
      {showSecondaryBadges && status && status.waiting > 0 && (
        <StatusBadge
          label={`+${status.waiting}`}
          variant='neutral'
          size='sm'
          copyable={false}
          aria-hidden='true'
        />
      )}
      {showSecondaryBadges && status?.cooling_down && (
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

export function ChannelConcurrencyStatusDisplay({
  status,
  maxConcurrency = status?.max_concurrency,
  showSecondaryBadges = true,
}: {
  status?: ChannelConcurrencyStatus
  maxConcurrency?: number
  showSecondaryBadges?: boolean
}) {
  const { t } = useTranslation()
  if (maxConcurrency === undefined) {
    return (
      <span
        className='text-muted-foreground text-xs'
        aria-label={t('Channel concurrency unavailable')}
      >
        -
      </span>
    )
  }

  return (
    <ChannelConcurrencyBadges
      maxConcurrency={maxConcurrency}
      status={status}
      showSecondaryBadges={showSecondaryBadges}
    />
  )
}

export function ChannelConcurrencyCell({
  channel,
  status,
  showSecondaryBadges = true,
}: {
  channel: Channel
  status?: ChannelConcurrencyStatus
  showSecondaryBadges?: boolean
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
  const [open, setOpen] = useState(false)

  const saveValue = useCallback(
    async (nextValue: number) => {
      if (nextValue === channel.max_concurrency) return
      setIsSaving(true)
      const saved = await handleUpdateChannelMaxConcurrency(
        channel.id,
        nextValue,
        queryClient
      )
      if (!saved) setDraftValue(channel.max_concurrency)
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

  if (isTagRow) return <ChannelConcurrencyStatusDisplay />
  if (!canEdit) {
    return (
      <ChannelConcurrencyStatusDisplay
        status={visibleStatus}
        maxConcurrency={channel.max_concurrency}
        showSecondaryBadges={showSecondaryBadges}
      />
    )
  }

  const triggerLabel =
    channel.max_concurrency > 0
      ? t('Edit maximum concurrency: {{value}}', {
          value: channel.max_concurrency,
        })
      : t('Set maximum concurrency')
  const trigger = (
    <Popover
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) updateScheduler.flush()
        setOpen(nextOpen)
      }}
    >
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='ghost'
            size='xs'
            aria-label={triggerLabel}
            className={cn(
              'h-auto rounded-sm p-0 font-normal underline decoration-dotted underline-offset-2',
              'hover:text-foreground hover:bg-transparent'
            )}
          />
        }
      >
        {channel.max_concurrency > 0 ? channel.max_concurrency : t('Unlimited')}
      </PopoverTrigger>
      <PopoverContent
        align='start'
        className='w-64 max-w-[calc(100vw-2rem)] gap-3 p-3'
      >
        <div className='flex items-center justify-between gap-3'>
          <PopoverTitle>{t('Maximum Concurrency')}</PopoverTitle>
          {isSaving && (
            <span className='text-muted-foreground flex items-center gap-1 text-xs'>
              <LoaderCircle className='size-3 animate-spin' />
              {t('Saving...')}
            </span>
          )}
        </div>
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
        <PopoverDescription>
          {t('Set to 0 for unlimited concurrency.')}
        </PopoverDescription>
      </PopoverContent>
    </Popover>
  )

  return (
    <ChannelConcurrencyBadges
      maxConcurrency={channel.max_concurrency}
      status={visibleStatus}
      maxTrigger={trigger}
      showSecondaryBadges={showSecondaryBadges}
    />
  )
}
