import { z } from 'zod'

import { tryJsonParse } from '../../utils/json-parser'

export type GroupModelRatioMap = Record<string, Record<string, number>>

function isGroupModelRatioMap(value: unknown): value is GroupModelRatioMap {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    return false
  }
  return Object.entries(value).every(
    ([group, modelRatios]) =>
      group.trim().length > 0 &&
      modelRatios !== null &&
      typeof modelRatios === 'object' &&
      !Array.isArray(modelRatios) &&
      Object.entries(modelRatios).every(
        ([model, ratio]) =>
          model.trim().length > 0 &&
          typeof ratio === 'number' &&
          Number.isFinite(ratio) &&
          ratio >= 0
      )
  )
}

export function parseGroupModelRatios(
  value: string
): GroupModelRatioMap | null {
  const parsed = tryJsonParse<unknown>(value)
  if (!parsed.success) return null
  return isGroupModelRatioMap(parsed.data) ? parsed.data : null
}

export function createGroupModelRatioSchema(
  translate: (key: string) => string
) {
  return z.string().refine((value) => parseGroupModelRatios(value) !== null, {
    message: translate(
      'Model-specific ratios must be a JSON map of group → model → non-negative finite number.'
    ),
  })
}

export function formatBillingModelName(modelName: string): string {
  const name = modelName.trim()
  for (const prefix of [
    'gemini-2.5-flash-lite',
    'gemini-2.5-flash',
    'gemini-2.5-pro',
  ]) {
    if (name.startsWith(prefix)) {
      return name.includes('-thinking-') ? `${prefix}-thinking-*` : name
    }
  }
  if (name.startsWith('gpt-4-gizmo')) return 'gpt-4-gizmo-*'
  if (name.startsWith('gpt-4o-gizmo')) return 'gpt-4o-gizmo-*'
  return name
}
