/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { Activity } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Bar, CartesianGrid, ComposedChart, Line, XAxis, YAxis } from 'recharts'

import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import { TitledCard } from '@/components/ui/titled-card'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import { getSensitiveWordTooltipTimestamp } from '../lib'
import type { SensitiveWordMonitorDashboard } from '../types'

export function SensitiveWordDispositionChart(props: {
  dashboard: SensitiveWordMonitorDashboard
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const isHourly = props.dashboard.bucket_seconds < 24 * 60 * 60
  const chartConfig = {
    blocked_requests: {
      label: t('Blocked'),
      color: 'var(--destructive)',
    },
    observed_requests: {
      label: t('Observed'),
      color: 'var(--chart-2)',
    },
    whitelist_bypasses: {
      label: t('Whitelist bypasses'),
      color: 'var(--chart-3)',
    },
    automatic_bans: {
      label: t('Automatic bans'),
      color: 'var(--chart-5)',
    },
  } satisfies ChartConfig
  const tickFormatter = (value: unknown) => {
    const timestamp =
      typeof value === 'number' || typeof value === 'string'
        ? Number(value)
        : Number.NaN
    if (!Number.isFinite(timestamp)) return '—'

    return new Intl.DateTimeFormat(locale, {
      month: 'short',
      day: 'numeric',
      ...(isHourly ? { hour: '2-digit' as const } : {}),
      timeZone: props.dashboard.timezone,
    }).format(new Date(timestamp * 1000))
  }

  return (
    <TitledCard
      title={t('Disposition trend')}
      description={t(
        'Blocked, observed, and whitelist-bypassed requests by Shanghai time'
      )}
      icon={<Activity />}
      iconTone='primary'
      disableHoverEffect
      contentClassName='px-2 pb-3 sm:px-4 sm:pb-4'
    >
      <ChartContainer
        config={chartConfig}
        className='h-[19rem] w-full sm:h-[21rem]'
      >
        <ComposedChart
          accessibilityLayer
          data={props.dashboard.series}
          margin={{ left: 0, right: 12, top: 12 }}
        >
          <CartesianGrid vertical={false} />
          <XAxis
            dataKey='timestamp'
            tickLine={false}
            axisLine={false}
            minTickGap={32}
            tickFormatter={tickFormatter}
          />
          <YAxis
            allowDecimals={false}
            tickLine={false}
            axisLine={false}
            width={36}
            tickFormatter={(value) => formatNumber(Number(value), locale)}
          />
          <ChartTooltip
            content={
              <ChartTooltipContent
                labelFormatter={(_value, payload) =>
                  tickFormatter(getSensitiveWordTooltipTimestamp(payload))
                }
                formatter={(value, name, item) => (
                  <>
                    <div
                      className='h-2.5 w-2.5 shrink-0 rounded-sm'
                      style={{ backgroundColor: item.color }}
                    />
                    <div className='flex min-w-0 flex-1 items-center justify-between gap-3 leading-none'>
                      <span className='text-muted-foreground'>
                        {chartConfig[name as keyof typeof chartConfig]?.label ??
                          String(name)}
                      </span>
                      <span className='text-foreground font-mono font-medium tabular-nums'>
                        {formatNumber(Number(value), locale)}
                      </span>
                    </div>
                  </>
                )}
              />
            }
          />
          <ChartLegend content={<ChartLegendContent />} />
          <Bar
            dataKey='blocked_requests'
            stackId='disposition'
            fill='var(--color-blocked_requests)'
            radius={[0, 0, 3, 3]}
          />
          <Bar
            dataKey='observed_requests'
            stackId='disposition'
            fill='var(--color-observed_requests)'
          />
          <Bar
            dataKey='whitelist_bypasses'
            stackId='disposition'
            fill='var(--color-whitelist_bypasses)'
            radius={[3, 3, 0, 0]}
          />
          <Line
            dataKey='automatic_bans'
            type='monotone'
            stroke='var(--color-automatic_bans)'
            strokeWidth={2}
            dot={false}
          />
        </ComposedChart>
      </ChartContainer>
    </TitledCard>
  )
}
