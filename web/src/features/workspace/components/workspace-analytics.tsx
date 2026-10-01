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
import { useEffect, useMemo, useState } from 'react'
import { VChart } from '@visactor/react-vchart'
import {
  BarChart3,
  ChartNoAxesColumn,
  PieChart,
  TrendingUp,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useThemeRadiusPx } from '@/lib/theme-radius'
import { cn } from '@/lib/utils'
import { VCHART_OPTION } from '@/lib/vchart'
import { useThemeCustomization } from '@/context/theme-customization-provider'
import { useTheme } from '@/context/theme-provider'
import { Skeleton } from '@/components/ui/skeleton'
import { processChartData } from '@/features/dashboard/lib'
import type { WorkspaceChartData, WorkspaceChartTab } from '../types'

let themeManagerPromise: Promise<
  (typeof import('@visactor/vchart'))['ThemeManager']
> | null = null

type ChartSpecKey =
  | 'spec_line'
  | 'spec_model_line'
  | 'spec_pie'
  | 'spec_rank_bar'

const CHART_SPEC_KEYS: Record<WorkspaceChartTab, ChartSpecKey> = {
  consumption: 'spec_line',
  trend: 'spec_model_line',
  distribution: 'spec_pie',
  ranking: 'spec_rank_bar',
}

const CHART_TABS = [
  { value: 'consumption', labelKey: 'Quota Distribution', icon: PieChart },
  { value: 'trend', labelKey: 'Call Trend', icon: TrendingUp },
  {
    value: 'distribution',
    labelKey: 'Call Count Distribution',
    icon: ChartNoAxesColumn,
  },
  { value: 'ranking', labelKey: 'Call Count Ranking', icon: BarChart3 },
] as const

export function WorkspaceAnalytics(props: WorkspaceChartData) {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()
  const { customization } = useThemeCustomization()
  const chartRadius = useThemeRadiusPx(
    '--radius-md',
    `${customization.preset}:${customization.radius}`
  )
  const [activeTab, setActiveTab] = useState<WorkspaceChartTab>('consumption')
  const [themeReady, setThemeReady] = useState(false)

  useEffect(() => {
    const updateTheme = async () => {
      setThemeReady(false)
      if (!themeManagerPromise) {
        themeManagerPromise = import('@visactor/vchart').then(
          (module) => module.ThemeManager
        )
      }
      const ThemeManager = await themeManagerPromise
      ThemeManager.setCurrentTheme(resolvedTheme === 'dark' ? 'dark' : 'light')
      setThemeReady(true)
    }
    void updateTheme()
  }, [resolvedTheme])

  const chartData = useMemo(
    () =>
      processChartData(
        props.loading ? [] : props.data,
        props.timeGranularity,
        t,
        chartRadius
      ),
    [
      chartRadius,
      customization.preset,
      props.data,
      props.loading,
      props.timeGranularity,
      t,
    ]
  )

  const spec = useMemo(() => {
    const baseSpec = chartData[CHART_SPEC_KEYS[activeTab]]
    if (activeTab !== 'ranking') return baseSpec
    return {
      ...baseSpec,
      direction: 'horizontal',
      xField: 'Count',
      yField: 'Model',
      legends: { visible: false },
      label: { visible: true, position: 'outside' },
    }
  }, [activeTab, chartData])

  const total =
    activeTab === 'consumption'
      ? chartData.totalQuotaDisplay
      : chartData.totalCountDisplay

  return (
    <section className='bg-card grid min-h-[24rem] overflow-hidden rounded-2xl border shadow-xs lg:grid-cols-[10rem_minmax(0,1fr)]'>
      <div
        className='bg-muted/20 flex overflow-x-auto border-b p-2 lg:flex-col lg:overflow-visible lg:border-r lg:border-b-0'
        role='tablist'
        aria-label={t('Model Call Analytics')}
      >
        {CHART_TABS.map((tab) => {
          const Icon = tab.icon
          const selected = activeTab === tab.value
          return (
            <button
              key={tab.value}
              type='button'
              role='tab'
              aria-selected={selected}
              onClick={() => setActiveTab(tab.value)}
              className={cn(
                'flex shrink-0 items-center gap-2 rounded-lg px-3 py-2 text-left text-sm font-medium transition-colors',
                selected
                  ? 'bg-accent text-accent-foreground'
                  : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground'
              )}
            >
              <Icon className='size-4 shrink-0' aria-hidden='true' />
              <span>{t(tab.labelKey)}</span>
            </button>
          )
        })}
      </div>

      <div className='flex min-w-0 flex-col'>
        <div className='flex items-center justify-between gap-4 border-b px-4 py-3 sm:px-5'>
          <div className='text-sm font-semibold'>
            {t(
              CHART_TABS.find((tab) => tab.value === activeTab)?.labelKey ??
                'Model Call Analytics'
            )}
          </div>
          <span className='text-muted-foreground text-xs'>
            {t('Total:')} {total}
          </span>
        </div>
        <div className='h-[21rem] min-w-0 p-2'>
          {props.loading || !themeReady ? (
            <Skeleton className='h-full w-full' />
          ) : (
            <VChart
              key={`${activeTab}-${props.data.length}-${resolvedTheme}-${customization.preset}`}
              spec={{
                ...spec,
                title: { visible: false },
                theme: resolvedTheme === 'dark' ? 'dark' : 'light',
                background: 'transparent',
              }}
              option={VCHART_OPTION}
            />
          )}
        </div>
      </div>
    </section>
  )
}
