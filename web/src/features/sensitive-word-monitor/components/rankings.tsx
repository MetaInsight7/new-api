/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { Boxes, ListFilter, Tags, UserRoundSearch } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Empty, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { TitledCard } from '@/components/ui/titled-card'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'

import type {
  SensitiveWordMonitorDimension,
  SensitiveWordMonitorTopRule,
  SensitiveWordMonitorTopUser,
} from '../types'

type RankingItem = {
  key: string
  label: string
  value: number
  detail?: ReactNode
}

function RankingCard(props: {
  title: string
  description: string
  icon: ReactNode
  iconTone: 'primary' | 'warning' | 'info' | 'chart-3'
  items: RankingItem[]
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const maximum = Math.max(...props.items.map((item) => item.value), 1)

  return (
    <TitledCard
      title={props.title}
      description={props.description}
      icon={props.icon}
      iconTone={props.iconTone}
      disableHoverEffect
      headerClassName='sm:p-4 sm:!pb-4'
      contentClassName='p-0'
      titleClassName='text-base sm:text-lg'
    >
      {props.items.length === 0 ? (
        <Empty className='py-10'>
          <EmptyHeader>
            <EmptyTitle>{t('No matches in this window')}</EmptyTitle>
          </EmptyHeader>
        </Empty>
      ) : (
        <ol className='divide-y'>
          {props.items.map((item, index) => (
            <li key={item.key} className='relative overflow-hidden px-4 py-3'>
              <div
                className='bg-primary/5 absolute inset-y-0 left-0'
                style={{ width: `${(item.value / maximum) * 100}%` }}
                aria-hidden='true'
              />
              <div className='relative flex items-start gap-3'>
                <span className='text-muted-foreground mt-0.5 w-4 shrink-0 text-xs font-medium tabular-nums'>
                  {formatNumber(index + 1, locale)}
                </span>
                <div className='min-w-0 flex-1'>
                  <div
                    className='truncate text-sm font-medium'
                    title={item.label}
                  >
                    {item.label}
                  </div>
                  {item.detail ? (
                    <div className='text-muted-foreground mt-1 text-xs'>
                      {item.detail}
                    </div>
                  ) : null}
                </div>
                <span className='shrink-0 font-mono text-sm font-semibold tabular-nums'>
                  {formatNumber(item.value, locale)}
                </span>
              </div>
            </li>
          ))}
        </ol>
      )}
    </TitledCard>
  )
}

function dimensionItems(
  values: SensitiveWordMonitorDimension[]
): RankingItem[] {
  return values.map((value) => ({
    key: value.name,
    label: value.name,
    value: value.count,
  }))
}

export function SensitiveWordRankings(props: {
  includeSensitiveMetadata: boolean
  rules: SensitiveWordMonitorTopRule[]
  users: SensitiveWordMonitorTopUser[]
  models: SensitiveWordMonitorDimension[]
  groups: SensitiveWordMonitorDimension[]
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const ruleItems = props.rules.map((rule) => ({
    key: rule.rule_id > 0 ? String(rule.rule_id) : rule.rule_name,
    label:
      rule.rule_name ||
      t('Rule #{{id}}', { id: formatNumber(rule.rule_id, locale) }),
    value: rule.match_count,
    detail: t('{{count}} blocked', {
      count: formatNumber(rule.blocked_count, locale),
    }),
  }))
  const userItems = props.users.map((user) => ({
    key: String(user.user_id),
    label:
      user.username ||
      t('User #{{id}}', { id: formatNumber(user.user_id, locale) }),
    value: user.match_count,
    detail: (
      <span className='flex flex-wrap items-center gap-1.5'>
        <span>
          {t('{{count}} blocked', {
            count: formatNumber(user.blocked_count, locale),
          })}
        </span>
        {user.automatic_bans > 0 ? (
          <Badge variant='destructive'>
            {t('{{count}} bans', {
              count: formatNumber(user.automatic_bans, locale),
            })}
          </Badge>
        ) : null}
      </span>
    ),
  }))

  return (
    <section
      className={cn(
        'grid gap-4 md:grid-cols-2',
        props.includeSensitiveMetadata ? '2xl:grid-cols-4' : 'xl:grid-cols-3'
      )}
      aria-label={t('Sensitive-word rankings')}
    >
      {props.includeSensitiveMetadata ? (
        <RankingCard
          title={t('Top matched rules')}
          description={t('Rules with the most matched requests')}
          icon={<ListFilter />}
          iconTone='primary'
          items={ruleItems}
        />
      ) : null}
      <RankingCard
        title={t('High-risk users')}
        description={t('Users with the most matched requests')}
        icon={<UserRoundSearch />}
        iconTone='warning'
        items={userItems}
      />
      <RankingCard
        title={t('Model sources')}
        description={t('Matched requests by requested model')}
        icon={<Boxes />}
        iconTone='info'
        items={dimensionItems(props.models)}
      />
      <RankingCard
        title={t('Group sources')}
        description={t('Matched requests by routing group')}
        icon={<Tags />}
        iconTone='chart-3'
        items={dimensionItems(props.groups)}
      />
    </section>
  )
}
