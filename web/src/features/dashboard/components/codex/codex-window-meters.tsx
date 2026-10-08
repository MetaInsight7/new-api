import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Progress } from '@/components/ui/progress'
import { getVisibleCodexLimitWindows } from '@/features/channels/lib/codex-usage-windows'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatTimestampToDate } from '@/lib/format'

import type { CodexLimitWindow } from '../../types'

function WindowMeter(props: { title: string; window: CodexLimitWindow }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const used = props.window.used_percent
  const percent =
    typeof used === 'number' && Number.isFinite(used)
      ? Math.min(100, Math.max(0, used))
      : null
  let tone: 'neutral' | 'info' | 'warning' | 'danger' = 'neutral'
  if (percent != null) {
    tone = 'info'
    if (percent >= 95) tone = 'danger'
    else if (percent >= 80) tone = 'warning'
  }
  const seconds = props.window.reset_after_seconds
  let resetsIn = '-'
  if (typeof seconds === 'number' && Number.isFinite(seconds) && seconds >= 0) {
    if (seconds >= 86400) {
      resetsIn = `${formatNumber(Math.floor(seconds / 86400), locale)} ${t('days')}`
    } else if (seconds >= 3600) {
      resetsIn = `${formatNumber(Math.floor(seconds / 3600), locale)}${t('h')} ${formatNumber(Math.floor((seconds % 3600) / 60), locale)}${t('m')}`
    } else {
      resetsIn = `${formatNumber(Math.floor(seconds / 60), locale)}${t('m')} ${formatNumber(Math.floor(seconds % 60), locale)}${t('s')}`
    }
  }
  return (
    <div className='min-w-0 rounded-lg border p-3'>
      <div className='mb-2 flex items-center justify-between gap-2'>
        <h4 className='text-sm font-medium'>{props.title}</h4>
        <StatusBadge
          label={percent == null ? '-' : `${formatNumber(percent, locale)}%`}
          variant={tone}
          copyable={false}
        />
      </div>
      {percent != null && (
        <Progress
          value={percent}
          aria-label={`${props.title}: ${formatNumber(percent, locale)}%`}
        />
      )}
      <dl className='text-muted-foreground mt-2 grid grid-cols-1 gap-1 text-xs'>
        <div>
          <dt className='inline'>{t('Reset at:')} </dt>
          <dd className='inline break-all'>
            {formatTimestampToDate(props.window.reset_at)}
          </dd>
        </div>
        <div>
          <dt className='inline'>{t('Resets in:')} </dt>
          <dd className='inline'>{resetsIn}</dd>
        </div>
        {props.window.limit_window_seconds != null && (
          <div>
            <dt className='inline'>{t('Window:')} </dt>
            <dd className='inline'>
              {formatNumber(props.window.limit_window_seconds, locale)} {t('s')}
            </dd>
          </div>
        )}
      </dl>
    </div>
  )
}

export function CodexWindowMeters(props: {
  fiveHourWindow?: CodexLimitWindow
  weeklyWindow?: CodexLimitWindow
  unknownWindows?: CodexLimitWindow[]
}) {
  const { t } = useTranslation()
  const windows = getVisibleCodexLimitWindows(
    props.fiveHourWindow,
    props.weeklyWindow,
    props.unknownWindows
  )
  const windowOccurrences = new Map<string, number>()
  if (windows.length === 0) {
    return (
      <p className='text-muted-foreground text-xs'>
        {t('No rate-limit windows returned')}
      </p>
    )
  }
  return (
    <div className='grid min-w-0 grid-cols-1 gap-3 md:grid-cols-2'>
      {windows.map((item) => {
        const identity = `${item.kind}-${JSON.stringify(item.window)}`
        const occurrence = windowOccurrences.get(identity) ?? 0
        windowOccurrences.set(identity, occurrence + 1)
        let title = t('Rate Limit Windows')
        if (item.kind === 'fiveHour') title = t('5-Hour Window')
        if (item.kind === 'weekly') title = t('Weekly Window')
        return (
          <WindowMeter
            key={`${identity}-${occurrence}`}
            title={title}
            window={item.window}
          />
        )
      })}
    </div>
  )
}
