/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { Link } from '@tanstack/react-router'
import {
  AlertTriangle,
  Calendar,
  RefreshCw,
  RotateCcw,
  Settings2,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DateTimePicker } from '@/components/datetime-picker'
import { Dialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { toIntlLocale } from '@/i18n/languages'
import { ROLE } from '@/lib/roles'
import { getRollingDateRange } from '@/lib/time'
import { useAuthStore } from '@/stores/auth-store'

import { SensitiveWordDispositionChart } from './components/disposition-chart'
import { SensitiveWordPolicyCard } from './components/policy-card'
import { SensitiveWordRankings } from './components/rankings'
import { SensitiveWordRecentEvents } from './components/recent-events'
import { SensitiveWordSummaryCards } from './components/summary-cards'
import {
  SensitiveWordMonitorError,
  SensitiveWordMonitorSkeleton,
} from './components/view-states'
import {
  useRefreshSensitiveWordMonitor,
  useSensitiveWordMonitorDashboard,
  useSensitiveWordMonitorEvents,
} from './hooks/use-sensitive-word-monitor'
import { sensitiveWordMonitorWindowLabel } from './lib'
import type {
  SensitiveWordMonitorEventFilters,
  SensitiveWordMonitorRange,
  SensitiveWordMonitorWindow,
} from './types'

const WINDOWS: SensitiveWordMonitorWindow[] = [24, 168, 720]
const DEFAULT_EVENT_FILTERS: SensitiveWordMonitorEventFilters = {
  page: 1,
  pageSize: 20,
  disposition: '',
  keyword: '',
}

export function SensitiveWordMonitor() {
  const { t, i18n } = useTranslation()
  const [range, setRange] = useState<SensitiveWordMonitorRange>({
    kind: 'preset',
    hours: 24,
  })
  const initialCustomRange = getRollingDateRange(7)
  const [customStart, setCustomStart] = useState<Date>(initialCustomRange.start)
  const [customEnd, setCustomEnd] = useState<Date>(initialCustomRange.end)
  const [customDraftStart, setCustomDraftStart] = useState<Date | undefined>()
  const [customDraftEnd, setCustomDraftEnd] = useState<Date | undefined>()
  const [customOpen, setCustomOpen] = useState(false)
  const [eventFilters, setEventFilters] = useState(DEFAULT_EVENT_FILTERS)
  const dashboardQuery = useSensitiveWordMonitorDashboard(range)
  const eventsQuery = useSensitiveWordMonitorEvents(range, eventFilters)
  const refresh = useRefreshSensitiveWordMonitor()
  const userRole = useAuthStore((state) => state.auth.user?.role)
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const generatedAt = dashboardQuery.data
    ? new Intl.DateTimeFormat(locale, {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
        timeZone: dashboardQuery.data.timezone,
      }).format(new Date(dashboardQuery.data.generated_at * 1000))
    : null
  const isRefreshing = dashboardQuery.isFetching || eventsQuery.isFetching

  const selectPreset = (hours: SensitiveWordMonitorWindow) => {
    setRange({ kind: 'preset', hours })
    setEventFilters((current) => ({ ...current, page: 1 }))
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
    if (
      customDraftEnd.getTime() - customDraftStart.getTime() >
      366 * 24 * 60 * 60 * 1000
    ) {
      toast.error(t('Time range cannot exceed 366 days'))
      return
    }
    setCustomStart(customDraftStart)
    setCustomEnd(customDraftEnd)
    setRange({
      kind: 'custom',
      start_timestamp: Math.floor(customDraftStart.getTime() / 1000),
      end_timestamp: Math.floor(customDraftEnd.getTime() / 1000),
    })
    setEventFilters((current) => ({ ...current, page: 1 }))
    setCustomOpen(false)
  }

  const rangeSelector = (
    <div className='bg-muted/60 flex max-w-full flex-nowrap justify-start gap-1 overflow-x-auto rounded-lg border p-0.5 md:justify-end'>
      {WINDOWS.map((windowHours) => (
        <Button
          key={windowHours}
          type='button'
          variant={
            range.kind === 'preset' && range.hours === windowHours
              ? 'default'
              : 'ghost'
          }
          size='sm'
          className='h-7 px-3 text-xs'
          onClick={() => selectPreset(windowHours)}
        >
          {t(sensitiveWordMonitorWindowLabel(windowHours))}
        </Button>
      ))}
      <Button
        type='button'
        variant={range.kind === 'custom' ? 'default' : 'ghost'}
        size='sm'
        className='h-7 px-3 text-xs'
        onClick={openCustomRange}
      >
        <Calendar data-icon='inline-start' />
        {t('Custom Time Range')}
      </Button>
    </div>
  )

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='flex min-w-0 flex-col gap-0.5 sm:flex-row sm:items-baseline sm:gap-3'>
          <span>{t('Sensitive-word monitor')}</span>
          <span className='text-muted-foreground truncate text-xs font-normal'>
            {generatedAt
              ? t('Generated {{time}} · {{timezone}}', {
                  time: generatedAt,
                  timezone: dashboardQuery.data?.timezone,
                })
              : t('Policy matches, enforcement, and audit traceability')}
          </span>
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <div className='hidden max-w-full md:block'>{rangeSelector}</div>
        <Button
          size='sm'
          variant='outline'
          disabled={isRefreshing}
          onClick={() => void refresh()}
        >
          <RefreshCw
            data-icon='inline-start'
            className={isRefreshing ? 'animate-spin' : undefined}
          />
          {isRefreshing ? t('Refreshing…') : t('Refresh')}
        </Button>
        {userRole === ROLE.SUPER_ADMIN ? (
          <Button
            size='sm'
            variant='outline'
            render={
              <Link
                to='/system-settings/request-policies/$section'
                params={{ section: 'filtering' }}
              />
            }
          >
            <Settings2 data-icon='inline-start' />
            {t('Rule settings')}
          </Button>
        ) : null}
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='mb-4 md:hidden'>{rangeSelector}</div>
        {dashboardQuery.isLoading ? <SensitiveWordMonitorSkeleton /> : null}
        {!dashboardQuery.data && dashboardQuery.isError ? (
          <SensitiveWordMonitorError
            isFetching={dashboardQuery.isFetching}
            onRetry={() => void dashboardQuery.refetch()}
          />
        ) : null}
        {dashboardQuery.data ? (
          <div className='flex flex-col gap-4'>
            {dashboardQuery.isError ? (
              <SensitiveWordMonitorError
                isFetching={dashboardQuery.isFetching}
                onRetry={() => void dashboardQuery.refetch()}
              />
            ) : null}
            {!dashboardQuery.data.policy.enabled ||
            !dashboardQuery.data.policy.check_prompt ? (
              <Alert className='border-warning/40 bg-warning/5 text-warning'>
                <AlertTriangle aria-hidden='true' />
                <AlertTitle>
                  {t('Sensitive-word enforcement is not fully active')}
                </AlertTitle>
                <AlertDescription>
                  {t(
                    'The enforcement policy or request-text inspection switch is disabled. Historical events remain visible, but new request text may not be checked.'
                  )}
                </AlertDescription>
              </Alert>
            ) : null}
            <SensitiveWordSummaryCards dashboard={dashboardQuery.data} />
            <section className='grid gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(19rem,1fr)]'>
              <SensitiveWordDispositionChart dashboard={dashboardQuery.data} />
              <SensitiveWordPolicyCard policy={dashboardQuery.data.policy} />
            </section>
            <SensitiveWordRankings
              includeSensitiveMetadata={
                dashboardQuery.data.sensitive_metadata_included
              }
              rules={dashboardQuery.data.top_rules}
              users={dashboardQuery.data.top_users}
              models={dashboardQuery.data.top_models}
              groups={dashboardQuery.data.top_groups}
            />
            <SensitiveWordRecentEvents
              events={eventsQuery.data?.items ?? []}
              total={eventsQuery.data?.total ?? 0}
              page={eventFilters.page}
              pageSize={eventFilters.pageSize}
              disposition={eventFilters.disposition}
              keyword={eventFilters.keyword}
              isLoading={eventsQuery.isLoading}
              isFetching={eventsQuery.isFetching}
              isError={eventsQuery.isError}
              includeSensitiveMetadata={
                dashboardQuery.data.sensitive_metadata_included
              }
              timezone={dashboardQuery.data.timezone}
              onFiltersChange={setEventFilters}
              onRetry={() => void eventsQuery.refetch()}
            />
          </div>
        ) : null}
        <Dialog
          open={customOpen}
          onOpenChange={setCustomOpen}
          title={t('Custom Time Range')}
          description={t(
            'Select the time range for sensitive-word monitoring.'
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
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
