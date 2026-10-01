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
import { useMemo, useState } from 'react'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Calendar, RotateCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  getRollingDateRange,
  getStartOfDay,
  type TimeGranularity,
} from '@/lib/time'
import { Button } from '@/components/ui/button'
import { DateTimePicker } from '@/components/datetime-picker'
import { Dialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout'
import { AnnouncementsPanel } from '@/features/dashboard/components/overview/announcements-panel'
import { UptimePanel } from '@/features/dashboard/components/overview/uptime-panel'
import { getWorkspaceUsage } from './api'
import { WorkspaceAnalytics } from './components/workspace-analytics'
import { WorkspaceApiAccess } from './components/workspace-api-access'
import { WorkspaceSummary } from './components/workspace-summary'

const RANGE_OPTIONS = [
  { key: 'today', labelKey: 'Today', timeGranularity: 'hour' },
  { key: 'last_24h', labelKey: 'Last 24 Hours', timeGranularity: 'hour' },
  { key: 'last_7d', labelKey: '7 Days', timeGranularity: 'day' },
  { key: 'last_30d', labelKey: '30 Days', timeGranularity: 'week' },
] as const

type WorkspacePreset = (typeof RANGE_OPTIONS)[number]
type WorkspaceRange =
  | WorkspacePreset
  | {
      key: 'custom'
      labelKey: 'Custom Time Range'
      timeGranularity: TimeGranularity
    }

function getRangeDates(range: WorkspacePreset) {
  const now = new Date()
  if (range.key === 'today') {
    return { start: getStartOfDay(now), end: now }
  }
  return getRollingDateRange(
    range.key === 'last_24h' ? 1 : range.key === 'last_7d' ? 7 : 30,
    now
  )
}

function getCustomGranularity(start: Date, end: Date): TimeGranularity {
  const durationDays = (end.getTime() - start.getTime()) / 86400000
  if (durationDays <= 1) return 'hour'
  if (durationDays <= 14) return 'day'
  return 'week'
}

export function Workspace() {
  const { t } = useTranslation()
  const [range, setRange] = useState<WorkspaceRange>(RANGE_OPTIONS[0])
  const [customOpen, setCustomOpen] = useState(false)
  const initialCustomRange = getRollingDateRange(7)
  const [customStart, setCustomStart] = useState<Date | undefined>(
    initialCustomRange.start
  )
  const [customEnd, setCustomEnd] = useState<Date | undefined>(
    initialCustomRange.end
  )
  const [customDraftStart, setCustomDraftStart] = useState<Date | undefined>()
  const [customDraftEnd, setCustomDraftEnd] = useState<Date | undefined>()
  const filters = useMemo(() => {
    const { start, end } =
      range.key === 'custom' && customStart && customEnd
        ? { start: customStart, end: customEnd }
        : getRangeDates(range as WorkspacePreset)
    return {
      start_timestamp: start,
      end_timestamp: end,
      time_granularity:
        range.key === 'custom'
          ? getCustomGranularity(start, end)
          : range.timeGranularity,
    }
  }, [customEnd, customStart, range])
  const usageQuery = useQuery({
    queryKey: ['workspace', 'usage', filters],
    queryFn: () => getWorkspaceUsage(filters),
    placeholderData: keepPreviousData,
    staleTime: 60 * 1000,
  })
  const recentTokensQuery = useQuery({
    queryKey: ['workspace', 'recent-tokens'],
    queryFn: () => {
      const { start, end } = getRollingDateRange(30)
      return getWorkspaceUsage({
        start_timestamp: start,
        end_timestamp: end,
        time_granularity: 'week',
      })
    },
    staleTime: 60 * 1000,
  })

  const chartData = {
    data: usageQuery.data?.data ?? [],
    loading: usageQuery.isLoading,
    totalQuota: usageQuery.data?.totalQuota ?? 0,
    totalCount: usageQuery.data?.totalCount ?? 0,
    totalTokens: usageQuery.data?.totalTokens ?? 0,
    recentTokens: recentTokensQuery.data?.totalTokens ?? 0,
    rangeKey: range.key,
    timeGranularity: filters.time_granularity,
  }

  const openCustomRange = () => {
    setCustomDraftStart(customStart)
    setCustomDraftEnd(customEnd)
    setCustomOpen(true)
  }

  const applyCustomRange = () => {
    if (!customDraftStart || !customDraftEnd) {
      toast.error(t('Please select a start and end time'))
      return
    }
    if (customDraftEnd <= customDraftStart) {
      toast.error(t('End time must be after start time'))
      return
    }
    setCustomStart(customDraftStart)
    setCustomEnd(customDraftEnd)
    setRange({
      key: 'custom',
      labelKey: 'Custom Time Range',
      timeGranularity: getCustomGranularity(customDraftStart, customDraftEnd),
    } as WorkspaceRange)
    setCustomOpen(false)
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Workspace')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <div className='bg-muted/60 flex flex-wrap justify-end gap-1 rounded-lg border p-0.5'>
          {RANGE_OPTIONS.map((option) => (
            <Button
              key={option.key}
              type='button'
              variant={range.key === option.key ? 'default' : 'ghost'}
              size='sm'
              className='h-7 px-3 text-xs'
              onClick={() => setRange(option)}
            >
              {t(option.labelKey)}
            </Button>
          ))}
          <Button
            type='button'
            variant={range.key === 'custom' ? 'default' : 'ghost'}
            size='sm'
            className='h-7 px-3 text-xs'
            onClick={openCustomRange}
          >
            <Calendar data-icon='inline-start' />
            {t('Custom Time Range')}
          </Button>
        </div>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <div className='grid items-stretch gap-4 xl:grid-cols-3'>
            <WorkspaceSummary {...chartData} />
            <WorkspaceApiAccess />
            <AnnouncementsPanel />
          </div>

          <div className='grid items-stretch gap-4 xl:grid-cols-[minmax(0,1fr)_20rem]'>
            <WorkspaceAnalytics {...chartData} />
            <UptimePanel />
          </div>
          <Dialog
            open={customOpen}
            onOpenChange={setCustomOpen}
            title={t('Custom Time Range')}
            description={t(
              'Select the time range for your workspace analytics.'
            )}
            contentClassName='sm:max-w-lg'
            footer={
              <>
                <Button
                  type='button'
                  variant='outline'
                  onClick={() => setCustomOpen(false)}
                >
                  {t('Cancel')}
                </Button>
                <Button type='button' onClick={applyCustomRange}>
                  {t('Apply Filters')}
                </Button>
              </>
            }
          >
            <div className='grid gap-4 py-2'>
              <div className='grid gap-2'>
                <label className='text-sm font-medium'>{t('Start Time')}</label>
                <DateTimePicker
                  value={customDraftStart}
                  onChange={setCustomDraftStart}
                  placeholder={t('Select start time')}
                />
              </div>
              <div className='grid gap-2'>
                <label className='text-sm font-medium'>{t('End Time')}</label>
                <DateTimePicker
                  value={customDraftEnd}
                  onChange={setCustomDraftEnd}
                  placeholder={t('Select end time')}
                />
              </div>
              <Button
                type='button'
                variant='ghost'
                className='justify-start'
                onClick={() => {
                  const defaults = getRollingDateRange(7)
                  setCustomDraftStart(defaults.start)
                  setCustomDraftEnd(defaults.end)
                }}
              >
                <RotateCcw data-icon='inline-start' />
                {t('Reset')}
              </Button>
            </div>
          </Dialog>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
