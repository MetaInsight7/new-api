import { z } from 'zod'

import { requireServerSuccess } from '@/lib/server-error-message'

import { updateSystemOption } from '../api'

const httpURL = z
  .string()
  .trim()
  .refine((value) => {
    if (!value) return true
    try {
      const url = new URL(value)
      return (
        ['http:', 'https:'].includes(url.protocol) &&
        !url.username &&
        !url.password
      )
    } catch {
      return false
    }
  }, 'Enter an HTTP or HTTPS URL without credentials')

export const monitorAlertSettingsSchema = z
  .object({
    dingtalk_alert_enabled: z.boolean(),
    dingtalk_alert_webhook_url: httpURL,
    dingtalk_alert_secret: z.string().trim(),
    dingtalk_alert_cooldown_minutes: z.number().int().min(1).max(153722867),
    ai_analysis_api_key: z.string().trim(),
    ai_analysis_base_url: httpURL,
    ai_analysis_model: z.string().trim(),
    clear_dingtalk_alert_secret: z.boolean(),
    clear_ai_analysis_api_key: z.boolean(),
  })
  .refine(
    (values) =>
      !values.dingtalk_alert_enabled || !!values.dingtalk_alert_webhook_url,
    {
      path: ['dingtalk_alert_webhook_url'],
      message: 'Configure the DingTalk webhook before enabling alerts',
    }
  )

export type MonitorAlertSettingsValues = z.infer<
  typeof monitorAlertSettingsSchema
>

export const monitorAlertSettingsDefaults: MonitorAlertSettingsValues = {
  dingtalk_alert_enabled: false,
  dingtalk_alert_webhook_url: '',
  dingtalk_alert_secret: '',
  dingtalk_alert_cooldown_minutes: 60,
  ai_analysis_api_key: '',
  ai_analysis_base_url: 'https://api.openai.com/v1',
  ai_analysis_model: 'gpt-5.4-mini',
  clear_dingtalk_alert_secret: false,
  clear_ai_analysis_api_key: false,
}

const secretFields = new Set(['dingtalk_alert_secret', 'ai_analysis_api_key'])

export function monitorAlertSettingsFromOptions(
  options: Record<string, unknown>
): MonitorAlertSettingsValues {
  const result = { ...monitorAlertSettingsDefaults }
  for (const field of Object.keys(result)) {
    if (secretFields.has(field) || field.startsWith('clear_')) continue
    const value = options[`monitor_setting.${field}`]
    if (value === undefined) continue
    const current = Reflect.get(result, field)
    if (typeof current === 'boolean') {
      Object.assign(result, { [field]: value === true || value === 'true' })
    } else if (typeof current === 'number') {
      if (Number.isFinite(Number(value))) {
        Object.assign(result, { [field]: Number(value) })
      }
    } else if (typeof value === 'string') {
      Object.assign(result, { [field]: value })
    }
  }
  return result
}

export async function saveMonitorAlertSettings(
  values: MonitorAlertSettingsValues,
  baseline: MonitorAlertSettingsValues,
  update: typeof updateSystemOption = updateSystemOption
): Promise<void> {
  const fields = (
    Object.keys(monitorAlertSettingsDefaults) as Array<
      keyof MonitorAlertSettingsValues
    >
  ).filter((field) => {
    if (field.startsWith('clear_')) return false
    if (secretFields.has(field)) {
      return !!Reflect.get(values, `clear_${field}`) || values[field] !== ''
    }
    return (
      values[field] !== baseline[field] ||
      field === 'ai_analysis_base_url' ||
      field === 'ai_analysis_model'
    )
  })
  fields.sort(
    (left, right) =>
      (Number(left === 'dingtalk_alert_enabled') -
        Number(right === 'dingtalk_alert_enabled')) *
      (values.dingtalk_alert_enabled ? 1 : -1)
  )
  for (const field of fields) {
    const value =
      secretFields.has(field) && Reflect.get(values, `clear_${field}`)
        ? ''
        : values[field]
    requireServerSuccess(
      await update({ key: `monitor_setting.${field}`, value })
    )
  }
}
