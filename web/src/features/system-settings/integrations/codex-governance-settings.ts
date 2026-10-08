import { z } from 'zod'

import { requireServerSuccess } from '@/lib/server-error-message'

import { updateSystemOption } from '../api'

export const codexGovernanceSettingsSchema = z.object({
  enabled: z.boolean(),
  probe_enabled: z.boolean(),
  probe_interval_minutes: z.number().int().min(60).max(525600),
  unsupported_message_patterns: z.string().min(1),
  official_source_urls: z.string(),
  official_lifecycle_terms: z.string(),
  alert_cooldown_minutes: z.number().int().min(1).max(525600),
  dingtalk_enabled: z.boolean(),
  dingtalk_webhook_url: z.string(),
  dingtalk_secret: z.string(),
  ai_analysis_api_key: z.string(),
  ai_analysis_base_url: z.string(),
  ai_analysis_model: z.string(),
  clear_dingtalk_secret: z.boolean(),
  clear_ai_analysis_api_key: z.boolean(),
})

export type CodexGovernanceSettingsValues = z.infer<
  typeof codexGovernanceSettingsSchema
>

export const codexGovernanceSettingsDefaults: CodexGovernanceSettingsValues = {
  enabled: false,
  probe_enabled: false,
  probe_interval_minutes: 1440,
  unsupported_message_patterns:
    "The '([^']+)' model is not supported when using Codex with a ChatGPT account\\.",
  official_source_urls: '',
  official_lifecycle_terms:
    'deprecated\nretired\nsunset\nunavailable\nnot supported',
  alert_cooldown_minutes: 60,
  dingtalk_enabled: false,
  dingtalk_webhook_url: '',
  dingtalk_secret: '',
  ai_analysis_api_key: '',
  ai_analysis_base_url: 'https://api.openai.com/v1',
  ai_analysis_model: 'gpt-5.4-mini',
  clear_dingtalk_secret: false,
  clear_ai_analysis_api_key: false,
}

const listFields = new Set([
  'unsupported_message_patterns',
  'official_source_urls',
  'official_lifecycle_terms',
])
const secretFields = new Set(['dingtalk_secret', 'ai_analysis_api_key'])

export function codexGovernanceLines(value: string): string[] {
  return value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
}

export function codexGovernanceSettingsFromOptions(
  options: Record<string, unknown>
): CodexGovernanceSettingsValues {
  const result = { ...codexGovernanceSettingsDefaults }
  for (const field of Object.keys(result)) {
    if (secretFields.has(field)) continue
    const value = options[`codex_model_governance_setting.${field}`]
    if (value === undefined) continue
    if (listFields.has(field)) {
      let parsed: unknown = value
      if (typeof parsed === 'string') {
        const serialized = parsed
        try {
          parsed = JSON.parse(serialized)
        } catch {
          parsed = codexGovernanceLines(serialized)
        }
      }
      if (
        Array.isArray(parsed) &&
        parsed.every((entry) => typeof entry === 'string')
      ) {
        Object.assign(result, { [field]: parsed.join('\n') })
      }
    } else if (typeof value === typeof Reflect.get(result, field)) {
      Object.assign(result, { [field]: value })
    } else if (typeof Reflect.get(result, field) === 'boolean') {
      Object.assign(result, { [field]: value === 'true' })
    } else if (
      typeof Reflect.get(result, field) === 'number' &&
      Number.isFinite(Number(value))
    ) {
      Object.assign(result, { [field]: Number(value) })
    }
  }
  return result
}

export async function saveCodexGovernanceSettings(
  values: CodexGovernanceSettingsValues,
  baseline: CodexGovernanceSettingsValues
): Promise<void> {
  const fields = (
    Object.keys(values) as Array<keyof CodexGovernanceSettingsValues>
  ).filter(
    (field) =>
      !field.startsWith('clear_') &&
      ((secretFields.has(field) && Reflect.get(values, `clear_${field}`)) ||
        (values[field] !== baseline[field] &&
          (!secretFields.has(field) || values[field] !== '')))
  )
  fields.sort(
    (left, right) =>
      (Number(left === 'enabled') - Number(right === 'enabled')) *
      (values.enabled ? 1 : -1)
  )
  for (const field of fields) {
    const raw =
      secretFields.has(field) && Reflect.get(values, `clear_${field}`)
        ? ''
        : values[field]
    const value =
      listFields.has(field) && typeof raw === 'string'
        ? JSON.stringify(codexGovernanceLines(raw))
        : raw
    requireServerSuccess(
      await updateSystemOption({
        key: `codex_model_governance_setting.${field}`,
        value,
      })
    )
  }
}
