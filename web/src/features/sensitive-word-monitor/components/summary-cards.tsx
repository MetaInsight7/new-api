/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import {
  Ban,
  Eye,
  ScanSearch,
  ShieldCheck,
  UserRoundX,
  Users,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatCard } from '@/features/dashboard/components/ui/stat-card'
import { toIntlLocale } from '@/i18n/languages'
import { formatCompactNumber } from '@/lib/format'

import type { SensitiveWordMonitorDashboard } from '../types'

export function SensitiveWordSummaryCards(props: {
  dashboard: SensitiveWordMonitorDashboard
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const { summary, series } = props.dashboard
  const cards = [
    {
      key: 'matched',
      title: t('Matched requests'),
      value: summary.matched_requests,
      description: t('Requests that matched at least one rule'),
      icon: ScanSearch,
      iconTone: 'primary' as const,
      sparkline: series.map((point) => point.matched_requests),
    },
    {
      key: 'blocked',
      title: t('Blocked'),
      value: summary.blocked_requests,
      description: t('Rejected before upstream routing'),
      icon: Ban,
      iconTone: 'destructive' as const,
      sparkline: series.map((point) => point.blocked_requests),
    },
    {
      key: 'observed',
      title: t('Observed'),
      value: summary.observed_requests,
      description: t('Audited without blocking'),
      icon: Eye,
      iconTone: 'info' as const,
      sparkline: series.map((point) => point.observed_requests),
    },
    {
      key: 'bypassed',
      title: t('Whitelist bypasses'),
      value: summary.whitelist_bypasses,
      description: t('Audited and allowed by whitelist'),
      icon: ShieldCheck,
      iconTone: 'success' as const,
      sparkline: series.map((point) => point.whitelist_bypasses),
    },
    {
      key: 'users',
      title: t('Matched users'),
      value: summary.affected_users,
      description: t(
        'Unique users with at least one match in the selected window'
      ),
      icon: Users,
      iconTone: 'warning' as const,
      sparkline: undefined,
    },
    {
      key: 'bans',
      title: t('Automatic bans'),
      value: summary.automatic_bans,
      description: t('Accounts disabled at the threshold'),
      icon: UserRoundX,
      iconTone: 'chart-5' as const,
      sparkline: series.map((point) => point.automatic_bans),
    },
  ]

  return (
    <section
      className='bg-card overflow-hidden rounded-xl border shadow-xs'
      aria-label={t('Sensitive-word summary')}
    >
      <div className='divide-border/60 grid grid-cols-2 divide-x sm:grid-cols-3 xl:grid-cols-6'>
        {cards.map((card) => (
          <div
            key={card.key}
            className='border-border/60 border-b p-3 sm:p-4 xl:border-b-0'
          >
            <StatCard
              compactMobile
              icon={card.icon}
              iconTone={card.iconTone}
              title={card.title}
              value={formatCompactNumber(card.value, locale)}
              description={card.description}
              sparkline={card.sparkline}
              sparklineVariant='line'
            />
          </div>
        ))}
      </div>
    </section>
  )
}
