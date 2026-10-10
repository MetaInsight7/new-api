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
import { useTranslation } from 'react-i18next'

import { StatusBadge, type StatusVariant } from '@/components/status-badge'

import type { ChannelConcurrencyStatus } from '../api'

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
