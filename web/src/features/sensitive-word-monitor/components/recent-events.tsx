/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { History, ScanSearch } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { TitledCard } from '@/components/ui/titled-card'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'

import { sensitiveWordDispositionLabel } from '../lib'
import type {
  SensitiveWordMonitorDisposition,
  SensitiveWordMonitorRecentEvent,
} from '../types'
import { SensitiveWordEventReviewSheet } from './event-review-sheet'

function DispositionBadge(props: {
  disposition: SensitiveWordMonitorDisposition
}) {
  const { t } = useTranslation()
  const label = t(sensitiveWordDispositionLabel(props.disposition))
  return (
    <Badge
      variant='outline'
      className={cn(
        props.disposition === 'block' &&
          'border-destructive/30 bg-destructive/10 text-destructive',
        props.disposition === 'observe' &&
          'border-info/30 bg-info/10 text-info',
        props.disposition === 'whitelist_bypass' &&
          'border-success/30 bg-success/10 text-success'
      )}
    >
      {label}
    </Badge>
  )
}

function EventReviewButton(props: {
  event: SensitiveWordMonitorRecentEvent
  onReview: (event: SensitiveWordMonitorRecentEvent) => void
}) {
  const { t } = useTranslation()
  return (
    <Button
      size='xs'
      variant='ghost'
      onClick={() => props.onReview(props.event)}
    >
      {t('Review and handle')}
      <ScanSearch data-icon='inline-end' />
    </Button>
  )
}

function RuleSummary(props: { names: string[] }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  if (props.names.length === 0) return <span>—</span>
  const visible = props.names.slice(0, 2)
  const remaining = props.names.length - visible.length

  return (
    <div className='flex min-w-0 flex-wrap gap-1'>
      {visible.map((name) => (
        <Badge key={name} variant='secondary' className='max-w-36 truncate'>
          {name}
        </Badge>
      ))}
      {remaining > 0 ? (
        <Badge variant='outline'>
          {t('+{{count}} more', { count: formatNumber(remaining, locale) })}
        </Badge>
      ) : null}
    </div>
  )
}

function eventUser(
  event: SensitiveWordMonitorRecentEvent,
  t: (key: string, options?: Record<string, unknown>) => string,
  locale: Intl.LocalesArgument
) {
  return (
    event.username ||
    t('User #{{id}}', {
      id: formatNumber(event.user_id, locale),
    })
  )
}

export function SensitiveWordRecentEvents(props: {
  events: SensitiveWordMonitorRecentEvent[]
  includeSensitiveMetadata: boolean
  timezone: string
}) {
  const { t, i18n } = useTranslation()
  const [selectedEvent, setSelectedEvent] =
    useState<SensitiveWordMonitorRecentEvent | null>(null)
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const dateFormatter = new Intl.DateTimeFormat(locale, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    timeZone: props.timezone,
  })

  return (
    <TitledCard
      title={t('Recent sensitive-word events')}
      description={t(
        'Review retained request evidence and handle affected accounts directly'
      )}
      icon={<History />}
      iconTone='primary'
      disableHoverEffect
      contentClassName='p-0'
    >
      {props.events.length === 0 ? (
        <Empty className='py-12'>
          <EmptyHeader>
            <EmptyTitle>{t('No sensitive-word events')}</EmptyTitle>
            <EmptyDescription>
              {t('No requests matched a configured rule in this window.')}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <>
          <div className='divide-y md:hidden'>
            {props.events.map((event) => (
              <article key={event.request_id} className='space-y-3 p-4'>
                <div className='flex items-start justify-between gap-3'>
                  <div className='min-w-0'>
                    <div className='truncate text-sm font-medium'>
                      {eventUser(event, t, locale)}
                    </div>
                    <div className='text-muted-foreground mt-0.5 text-xs'>
                      {dateFormatter.format(new Date(event.created_at * 1000))}
                    </div>
                  </div>
                  <DispositionBadge disposition={event.disposition} />
                </div>
                <div className='text-muted-foreground grid grid-cols-2 gap-2 text-xs'>
                  <div className='min-w-0'>
                    <span className='block'>{t('Model')}</span>
                    <span className='text-foreground block truncate font-mono'>
                      {event.model_name || '—'}
                    </span>
                  </div>
                  <div className='min-w-0'>
                    <span className='block'>{t('Group')}</span>
                    <span className='text-foreground block truncate'>
                      {event.group_name || '—'}
                    </span>
                  </div>
                </div>
                {props.includeSensitiveMetadata ? (
                  <RuleSummary names={event.rule_names ?? []} />
                ) : null}
                <div className='flex items-center justify-between gap-3'>
                  <span className='text-muted-foreground text-xs'>
                    {t('Violation count: {{count}}', {
                      count: formatNumber(event.violation_count, locale),
                    })}
                  </span>
                  <EventReviewButton
                    event={event}
                    onReview={setSelectedEvent}
                  />
                </div>
              </article>
            ))}
          </div>
          <div className='hidden overflow-x-auto md:block'>
            <Table
              className={cn(
                props.includeSensitiveMetadata
                  ? 'min-w-[66rem]'
                  : 'min-w-[50rem]'
              )}
            >
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Time')}</TableHead>
                  <TableHead>{t('Disposition')}</TableHead>
                  <TableHead>{t('User')}</TableHead>
                  <TableHead>{t('Model / Group')}</TableHead>
                  {props.includeSensitiveMetadata ? (
                    <TableHead>{t('Matched rules')}</TableHead>
                  ) : null}
                  <TableHead className='text-right'>
                    {t('Violations')}
                  </TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {props.events.map((event) => (
                  <TableRow key={event.request_id}>
                    <TableCell className='text-muted-foreground whitespace-nowrap'>
                      {dateFormatter.format(new Date(event.created_at * 1000))}
                    </TableCell>
                    <TableCell>
                      <div className='flex items-center gap-2'>
                        <DispositionBadge disposition={event.disposition} />
                        {event.automatic_ban ? (
                          <Badge variant='destructive'>
                            {t('Auto-banned')}
                          </Badge>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div
                        className='max-w-40 truncate font-medium'
                        title={eventUser(event, t, locale)}
                      >
                        {eventUser(event, t, locale)}
                      </div>
                      <div className='text-muted-foreground text-xs tabular-nums'>
                        #{formatNumber(event.user_id, locale)}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div
                        className='max-w-56 truncate font-mono text-xs'
                        title={event.model_name}
                      >
                        {event.model_name || '—'}
                      </div>
                      <div
                        className='text-muted-foreground mt-0.5 max-w-56 truncate text-xs'
                        title={event.group_name}
                      >
                        {event.group_name || '—'}
                      </div>
                    </TableCell>
                    {props.includeSensitiveMetadata ? (
                      <TableCell>
                        <RuleSummary names={event.rule_names ?? []} />
                      </TableCell>
                    ) : null}
                    <TableCell className='text-right font-mono tabular-nums'>
                      {formatNumber(event.violation_count, locale)}
                    </TableCell>
                    <TableCell className='text-right'>
                      <EventReviewButton
                        event={event}
                        onReview={setSelectedEvent}
                      />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </>
      )}
      <SensitiveWordEventReviewSheet
        event={selectedEvent}
        onOpenChange={(open) => {
          if (!open) setSelectedEvent(null)
        }}
      />
    </TitledCard>
  )
}
