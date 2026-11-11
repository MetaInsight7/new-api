/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { TitledCard } from '@/components/ui/titled-card'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { SensitiveWordMonitorPolicy } from '../types'

function PolicyRow(props: {
  label: string
  value: React.ReactNode
  detail?: string
}) {
  return (
    <div className='flex items-start justify-between gap-4 border-b py-3 last:border-b-0'>
      <div className='min-w-0'>
        <div className='text-sm font-medium'>{props.label}</div>
        {props.detail ? (
          <div className='text-muted-foreground mt-0.5 text-xs'>
            {props.detail}
          </div>
        ) : null}
      </div>
      <div className='shrink-0 text-right text-sm'>{props.value}</div>
    </div>
  )
}

function StatusBadge(props: { enabled: boolean }) {
  const { t } = useTranslation()
  return (
    <Badge
      variant='outline'
      className={cn(
        props.enabled
          ? 'border-success/30 bg-success/10 text-success'
          : 'text-muted-foreground'
      )}
    >
      {props.enabled ? t('Enabled') : t('Disabled')}
    </Badge>
  )
}

export function SensitiveWordPolicyCard(props: {
  policy: SensitiveWordMonitorPolicy
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const activeRules =
    props.policy.blocking_rule_count + props.policy.observe_rule_count

  return (
    <TitledCard
      title={t('Current policy')}
      description={t('Effective enforcement and evidence settings')}
      icon={<ShieldCheck />}
      iconTone={props.policy.enabled ? 'success' : 'warning'}
      disableHoverEffect
      contentClassName='py-0'
    >
      <PolicyRow
        label={t('Sensitive-word enforcement')}
        detail={t('Matching, auditing, blocking, and violation counting')}
        value={<StatusBadge enabled={props.policy.enabled} />}
      />
      <PolicyRow
        label={t('Request text inspection')}
        detail={t('Checks supported model-visible request text')}
        value={<StatusBadge enabled={props.policy.check_prompt} />}
      />
      <PolicyRow
        label={t('Rule snapshot')}
        detail={t('{{block}} blocking · {{observe}} observe', {
          block: formatNumber(props.policy.blocking_rule_count, locale),
          observe: formatNumber(props.policy.observe_rule_count, locale),
        })}
        value={t('{{active}} / {{total}} active', {
          active: formatNumber(activeRules, locale),
          total: formatNumber(props.policy.rule_count, locale),
        })}
      />
      <PolicyRow
        label={t('Automatic enforcement')}
        detail={t('Disables accounts after counted violations')}
        value={
          props.policy.auto_ban_enabled ? (
            <span className='font-medium tabular-nums'>
              {t('{{count}} matches', {
                count: formatNumber(props.policy.ban_threshold, locale),
              })}
            </span>
          ) : (
            <StatusBadge enabled={false} />
          )
        }
      />
      <PolicyRow
        label={t('Evidence retention')}
        detail={t('Full evidence remains permission-protected')}
        value={t('{{count}} days', {
          count: formatNumber(props.policy.full_prompt_retention_days, locale),
        })}
      />
    </TitledCard>
  )
}
