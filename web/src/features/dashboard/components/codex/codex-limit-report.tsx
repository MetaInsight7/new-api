import { RefreshIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { CircleDollarSign, Hash, LineChart } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { toIntlLocale } from '@/i18n/languages'
import {
  formatCompactNumber,
  formatNumber,
  formatQuota,
  formatTimestampToDate,
} from '@/lib/format'
import {
  createServerError,
  getServerErrorMessage,
  requireServerSuccess,
} from '@/lib/server-error-message'
import { cn } from '@/lib/utils'

import { getCodexLimitReport } from '../../api'
import { TIME_RANGE_PRESETS } from '../../constants'
import { StatCard } from '../ui/stat-card'
import { CodexChannelReport } from './codex-channel-report'
import {
  buildCodexLimitReportTimeRange,
  isSameCodexLimitReportTimeRange,
} from './codex-limit-report-range'

export function CodexLimitReportPanel() {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [selectedRange, setSelectedRange] = useState(7)
  const [timeRange, setTimeRange] = useState(() =>
    buildCodexLimitReportTimeRange(7)
  )
  const query = useQuery({
    queryKey: ['dashboard', 'codex-limit-report', timeRange],
    queryFn: async ({ signal }) => {
      const response = requireServerSuccess(
        await getCodexLimitReport(timeRange, signal)
      )
      if (!response.data || !Array.isArray(response.data.rows)) {
        throw createServerError(response, t('Invalid Codex report response'))
      }
      return response.data
    },
    staleTime: 30_000,
    retry: false,
    meta: { errorToast: false },
  })
  const handleRefresh = (): void => {
    const nextRange = buildCodexLimitReportTimeRange(selectedRange)
    if (isSameCodexLimitReportTimeRange(timeRange, nextRange)) {
      void query.refetch()
    } else {
      setTimeRange(nextRange)
    }
  }
  const report = query.data
  return (
    <div className='flex min-w-0 flex-col gap-4'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div
          className='flex flex-wrap items-center gap-1.5'
          aria-label={t('Time range')}
        >
          {TIME_RANGE_PRESETS.map((preset) => (
            <Button
              key={preset.days}
              type='button'
              variant={selectedRange === preset.days ? 'default' : 'outline'}
              size='sm'
              aria-pressed={selectedRange === preset.days}
              onClick={() => {
                setSelectedRange(preset.days)
                setTimeRange(buildCodexLimitReportTimeRange(preset.days))
              }}
            >
              {t(preset.label)}
            </Button>
          ))}
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={handleRefresh}
          disabled={query.isFetching}
        >
          <HugeiconsIcon
            icon={RefreshIcon}
            aria-hidden='true'
            data-icon='inline-start'
            className={cn(query.isFetching && 'animate-spin')}
          />
          {t('Refresh')}
        </Button>
      </div>
      <div className='text-muted-foreground flex flex-col gap-1 text-xs'>
        <p>
          {formatTimestampToDate(
            report?.start_timestamp ?? timeRange.start_timestamp
          )}{' '}
          —{' '}
          {formatTimestampToDate(
            report?.end_timestamp ?? timeRange.end_timestamp
          )}
        </p>
        <p>
          {t(
            'Upstream limits are a current snapshot; local usage follows the selected time range.'
          )}
        </p>
        {report && (
          <p>
            {t('Updated at')}: {formatTimestampToDate(report.generated_at)}
          </p>
        )}
      </div>
      {query.isPending && (
        <LoadingState message={t('Loading Codex limits...')} />
      )}
      {query.isError && (
        <ErrorState
          title={t('Failed to fetch Codex limits')}
          description={getServerErrorMessage(query.error)}
          onRetry={handleRefresh}
        />
      )}
      {report && (
        <>
          <div className='grid grid-cols-1 gap-3 lg:grid-cols-3'>
            <StatCard
              icon={Hash}
              title={t('Codex Channels')}
              value={formatNumber(report.total_channels, locale)}
              description={t('Query results')}
              details={[
                {
                  label: t('Query succeeded'),
                  value: formatNumber(report.success_count, locale),
                  tone: 'success',
                },
                {
                  label: t('Query failed'),
                  value: formatNumber(report.failure_count, locale),
                  tone: 'destructive',
                },
              ]}
            />
            <StatCard
              icon={LineChart}
              title={t('Total Tokens')}
              value={formatCompactNumber(report.total_token_used, locale)}
              description={t('Selected range usage')}
            />
            <StatCard
              icon={CircleDollarSign}
              title={t('Amount')}
              value={formatQuota(report.total_quota)}
              description={t('Selected range usage')}
            />
          </div>
          <CodexChannelReport rows={report.rows} />
        </>
      )}
    </div>
  )
}
