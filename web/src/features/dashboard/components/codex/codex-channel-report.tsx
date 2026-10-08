import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { StatusBadge } from '@/components/status-badge'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { CHANNEL_STATUS_CONFIG } from '@/features/channels/constants'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatQuota } from '@/lib/format'

import type { CodexLimitReportRow } from '../../types'
import { CodexWindowMeters } from './codex-window-meters'

function ChannelPanel(props: { row: CodexLimitReportRow }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const row = props.row
  const additionalLimitOccurrences = new Map<string, number>()
  const channelStatus =
    CHANNEL_STATUS_CONFIG[
      row.channel_status as keyof typeof CHANNEL_STATUS_CONFIG
    ]
  let availability = t('Unknown')
  let availabilityTone: 'neutral' | 'success' | 'danger' = 'neutral'
  if (row.limit_reached || row.allowed === false) {
    availability = t('Limited')
    availabilityTone = 'danger'
  } else if (row.allowed === true) {
    availability = t('Available')
    availabilityTone = 'success'
  }
  return (
    <Card className='min-w-0'>
      <CardHeader>
        <CardTitle className='break-all'>
          {row.channel_name || `#${row.channel_id}`}
        </CardTitle>
        <div className='flex flex-wrap gap-2'>
          <StatusBadge
            label={row.success ? t('Query succeeded') : t('Query failed')}
            variant={row.success ? 'success' : 'danger'}
            copyable={false}
          />
          <StatusBadge
            label={channelStatus ? t(channelStatus.label) : t('Unknown')}
            variant={channelStatus?.variant ?? 'neutral'}
            copyable={false}
          />
          {row.success && (
            <StatusBadge
              label={availability}
              variant={availabilityTone}
              copyable={false}
            />
          )}
          {row.plan_type && (
            <StatusBadge
              label={row.plan_type}
              variant='info'
              copyable={false}
            />
          )}
        </div>
      </CardHeader>
      <CardContent className='flex min-w-0 flex-col gap-4'>
        <dl className='text-muted-foreground grid grid-cols-1 gap-2 text-xs sm:grid-cols-2'>
          <div>
            <dt>{t('Channel')}</dt>
            <dd>#{formatNumber(row.channel_id, locale)}</dd>
          </div>
          {row.upstream_status != null && (
            <div>
              <dt>HTTP</dt>
              <dd>{row.upstream_status}</dd>
            </div>
          )}
          {row.email && (
            <div>
              <dt>{t('Email')}</dt>
              <dd className='break-all'>{row.email}</dd>
            </div>
          )}
          {row.account_id && (
            <div>
              <dt>{t('Account ID')}</dt>
              <dd className='font-mono break-all'>{row.account_id}</dd>
            </div>
          )}
          {row.user_id && (
            <div>
              <dt>{t('User ID')}</dt>
              <dd className='font-mono break-all'>{row.user_id}</dd>
            </div>
          )}
        </dl>
        {!row.success && (
          <Alert variant='destructive'>
            <AlertTitle>{t('Query failed')}</AlertTitle>
            <AlertDescription>
              {row.message ? t(row.message) : t('Failed to fetch Codex usage')}
            </AlertDescription>
          </Alert>
        )}
        <dl className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
          <div className='rounded-lg border p-3'>
            <dt className='text-muted-foreground text-xs'>
              {t('Range Tokens')}
            </dt>
            <dd className='mt-1 text-xl font-semibold tabular-nums'>
              {formatNumber(row.range_token_used, locale)}
            </dd>
          </div>
          <div className='rounded-lg border p-3'>
            <dt className='text-muted-foreground text-xs'>
              {t('Range Amount')}
            </dt>
            <dd className='mt-1 text-xl font-semibold tabular-nums'>
              {formatQuota(row.range_quota)}
            </dd>
          </div>
        </dl>
        {row.success && (
          <section className='flex min-w-0 flex-col gap-2'>
            <h3 className='text-sm font-semibold'>{t('Base Limits')}</h3>
            <CodexWindowMeters
              fiveHourWindow={row.base_five_hour_window}
              weeklyWindow={row.base_weekly_window}
              unknownWindows={row.unknown_windows}
            />
          </section>
        )}
        {(row.additional_limits?.length ?? 0) > 0 && (
          <section className='flex min-w-0 flex-col gap-3'>
            <h3 className='text-sm font-semibold'>
              {t('Additional Restrictions')}
            </h3>
            {row.additional_limits?.map((item) => {
              const identity = `${item.name}-${item.metered_feature ?? ''}`
              const occurrence = additionalLimitOccurrences.get(identity) ?? 0
              additionalLimitOccurrences.set(identity, occurrence + 1)
              return (
                <div
                  key={`${identity}-${occurrence}`}
                  className='bg-muted/30 flex min-w-0 flex-col gap-2 rounded-lg p-3'
                >
                  <h4 className='text-sm font-medium break-all'>
                    {item.name || t('Additional Restriction')}
                  </h4>
                  {item.metered_feature && (
                    <StatusBadge
                      label={item.metered_feature}
                      variant='neutral'
                      copyable={false}
                      className='max-w-full'
                    />
                  )}
                  <CodexWindowMeters
                    fiveHourWindow={item.five_hour_window}
                    weeklyWindow={item.weekly_window}
                    unknownWindows={item.unknown_windows}
                  />
                </div>
              )
            })}
          </section>
        )}
      </CardContent>
    </Card>
  )
}

export function CodexChannelReport(props: { rows: CodexLimitReportRow[] }) {
  const { t } = useTranslation()
  const [selectedChannel, setSelectedChannel] = useState('')
  if (props.rows.length === 0) {
    return <EmptyState title={t('No Codex channels found')} />
  }
  const activeChannel = props.rows.some(
    (row) => String(row.channel_id) === selectedChannel
  )
    ? selectedChannel
    : String(props.rows[0].channel_id)
  return (
    <Tabs
      value={activeChannel}
      onValueChange={setSelectedChannel}
      className='min-w-0 gap-3'
    >
      <TabsList
        aria-label={t('Codex Channels')}
        className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'
      >
        {props.rows.map((row) => (
          <TabsTrigger
            key={row.channel_id}
            value={String(row.channel_id)}
            className='h-auto min-h-8 max-w-52'
          >
            <span className='truncate'>
              {row.channel_name || `#${row.channel_id}`}
            </span>
            {!row.success && (
              <span className='text-destructive' aria-label={t('Query failed')}>
                !
              </span>
            )}
          </TabsTrigger>
        ))}
      </TabsList>
      {props.rows.map((row) => (
        <TabsContent key={row.channel_id} value={String(row.channel_id)}>
          <ChannelPanel row={row} />
        </TabsContent>
      ))}
    </Tabs>
  )
}
