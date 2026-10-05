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
import { Activity, Coins, Layers, WalletCards, type LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { formatQuota } from '@/lib/format'
import { toIntlLocale } from '@/i18n/languages'
import { Skeleton } from '@/components/ui/skeleton'
import type { WorkspaceChartData } from '../types'

interface SummaryItem {
  label: string
  value: string
  description: string
  descriptionClassName?: string
  icon: LucideIcon
}

function formatWorkspaceCompactNumber(
  value: number | null | undefined,
  locale?: string
): string {
  if (value == null || Number.isNaN(value)) return '-'
  return Intl.NumberFormat(toIntlLocale(locale), {
    notation: 'compact',
    compactDisplay: 'short',
    maximumFractionDigits: 1,
  }).format(value)
}

function getRangeLabel(rangeKey: string, t: (key: string) => string) {
  switch (rangeKey) {
    case 'last_24h':
      return t('Last 24 Hours')
    case 'last_7d':
      return t('Last 7 Days for summary')
    case 'last_30d':
      return t('Last 30 Days for summary')
    case 'custom':
      return t('Custom Time Range')
    default:
      return t('Today for summary')
  }
}

function getBalanceStatus(quota: number, setting: string | undefined, t: (key: string) => string) {
  if (quota <= 0) return { label: t('Insufficient balance'), tone: 'text-destructive' }

  try {
    const threshold = Number(JSON.parse(setting || '{}').quota_warning_threshold || 500000)
    if (quota <= threshold) return { label: t('Low balance'), tone: 'text-amber-600 dark:text-amber-400' }
  } catch {
    // Use the standard threshold when a legacy user setting cannot be parsed.
  }

  return { label: t('Available'), tone: 'text-success' }
}

export function WorkspaceSummary(props: WorkspaceChartData) {
  const { t, i18n } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const rangeLabel = getRangeLabel(props.rangeKey, t)
  const balance = Number(user?.quota ?? 0)
  const userSetting = typeof user?.setting === 'string' ? user.setting : undefined
  const balanceStatus = getBalanceStatus(balance, userSetting, t)
  const isCustomRange = props.rangeKey === 'custom'
  const items: SummaryItem[] = [
    {
      label: t('Current Balance'),
      value: formatQuota(balance),
      description: balanceStatus.label,
      descriptionClassName: balanceStatus.tone,
      icon: WalletCards,
    },
    {
      label: isCustomRange
        ? t('Custom Range Consumed')
        : t('Range Consumption', { range: rangeLabel }),
      value: formatQuota(props.totalQuota),
      description: `${t('Historical consumption')} ${formatQuota(Number(user?.used_quota ?? 0))}`,
      icon: Coins,
    },
    {
      label: isCustomRange
        ? t('Custom Range Requested')
        : t('Range Requests', { range: rangeLabel }),
      value: formatWorkspaceCompactNumber(props.totalCount, locale),
      description: `${t('Historical requests')} ${formatWorkspaceCompactNumber(Number(user?.request_count ?? 0), locale)}`,
      icon: Activity,
    },
    {
      label: isCustomRange
        ? t('Custom Range Tokens')
        : t('Workspace Range Tokens', { range: rangeLabel }),
      value: formatWorkspaceCompactNumber(props.totalTokens, locale),
      description: `${t('Last 30 days total')} ${formatWorkspaceCompactNumber(props.recentTokens, locale)}`,
      icon: Layers,
    },
  ]

  return (
    <section className='bg-card h-full overflow-hidden rounded-2xl border shadow-xs'>
      <div className='border-b px-4 py-3 sm:px-5'>
        <div className='flex items-center gap-2 text-sm font-semibold'>
          <Activity className='text-primary size-4' aria-hidden='true' />
          {t('Usage at a glance')}
        </div>
        <div className='text-muted-foreground mt-1 text-xs'>
          {t('Monitor balance, usage, and request volume')}
        </div>
      </div>
      <div className='grid grid-cols-2 gap-3 p-3 sm:p-4'>
        {items.map((item) => {
          const Icon = item.icon
          return (
            <div
              key={item.label}
              className='bg-muted/20 flex min-h-28 flex-col justify-between rounded-xl border p-3 sm:p-4'
            >
              <div className='text-muted-foreground flex items-center gap-2 text-xs font-medium'>
                <Icon className='text-primary size-4 shrink-0' aria-hidden='true' />
                <span className='truncate'>{item.label}</span>
              </div>
              {props.loading ? (
                <div className='mt-4 grid gap-1.5'>
                  <Skeleton className='h-7 w-20' />
                  <Skeleton className='h-3 w-28' />
                </div>
              ) : (
                <div className='mt-4 min-w-0'>
                  <div className='font-mono text-xl font-semibold tracking-tight tabular-nums sm:text-2xl'>
                    {item.value}
                  </div>
                  <div className={`mt-1 truncate text-xs ${item.descriptionClassName ?? 'text-muted-foreground'}`}>
                    {item.description}
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </section>
  )
}
