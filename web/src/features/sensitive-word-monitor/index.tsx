/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { Link } from '@tanstack/react-router'
import { AlertTriangle, RefreshCw, Settings2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { toIntlLocale } from '@/i18n/languages'
import { ROLE } from '@/lib/roles'
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
} from './hooks/use-sensitive-word-monitor'
import { sensitiveWordMonitorWindowLabel } from './lib'
import type { SensitiveWordMonitorWindow } from './types'

const WINDOWS: SensitiveWordMonitorWindow[] = [24, 168, 720]

export function SensitiveWordMonitor() {
  const { t, i18n } = useTranslation()
  const [hours, setHours] = useState<SensitiveWordMonitorWindow>(24)
  const dashboardQuery = useSensitiveWordMonitorDashboard(hours)
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

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='flex min-w-0 flex-col gap-0.5 sm:flex-row sm:items-baseline sm:gap-3'>
          <span>{t('Sensitive-word monitor')}</span>
          <span className='text-muted-foreground truncate text-xs font-normal'>
            {generatedAt
              ? t('Generated {{time}} · Asia/Shanghai', { time: generatedAt })
              : t('Policy matches, enforcement, and audit traceability')}
          </span>
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <ToggleGroup
          value={[String(hours)]}
          variant='outline'
          size='sm'
          aria-label={t('Sensitive-word time window')}
          onValueChange={(values) => {
            const next = Number(values[0])
            if (next === 24 || next === 168 || next === 720) setHours(next)
          }}
        >
          {WINDOWS.map((windowHours) => (
            <ToggleGroupItem key={windowHours} value={String(windowHours)}>
              {t(sensitiveWordMonitorWindowLabel(windowHours))}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
        <Button
          size='sm'
          variant='outline'
          disabled={dashboardQuery.isFetching}
          onClick={() => void refresh()}
        >
          <RefreshCw
            data-icon='inline-start'
            className={dashboardQuery.isFetching ? 'animate-spin' : undefined}
          />
          {dashboardQuery.isFetching ? t('Refreshing…') : t('Refresh')}
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
              events={dashboardQuery.data.recent_events}
              includeSensitiveMetadata={
                dashboardQuery.data.sensitive_metadata_included
              }
              timezone={dashboardQuery.data.timezone}
            />
          </div>
        ) : null}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
