import { z } from 'zod'

import { tryJsonParse } from '../../utils/json-parser'

export type GroupModelRatioMap = Record<string, Record<string, number>>

export type GroupModelRatioBatchAction =
  | { type: 'set'; ratio: number; overwrite: boolean }
  | { type: 'remove' }

export type GroupModelRatioBatchResult = {
  ratios: GroupModelRatioMap
  billingKeys: string[]
  added: number
  updated: number
  removed: number
  unchanged: number
  collapsed: number
}

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

export function applyGroupModelRatioBatch(input: {
  ratios: GroupModelRatioMap
  group: string
  modelNames: string[]
  action: GroupModelRatioBatchAction
}): GroupModelRatioBatchResult {
  const billingKeys = [
    ...new Set(input.modelNames.map(formatBillingModelName).filter(Boolean)),
  ]
  const currentGroup = Object.hasOwn(input.ratios, input.group)
    ? input.ratios[input.group]
    : {}
  const nextGroup = { ...currentGroup }
  let added = 0
  let updated = 0
  let removed = 0
  let unchanged = 0

  for (const billingKey of billingKeys) {
    const configured = Object.hasOwn(currentGroup, billingKey)
    if (input.action.type === 'remove') {
      if (!configured) {
        unchanged += 1
        continue
      }
      delete nextGroup[billingKey]
      removed += 1
      continue
    }
    if (!configured) {
      nextGroup[billingKey] = input.action.ratio
      added += 1
      continue
    }
    if (
      input.action.overwrite &&
      currentGroup[billingKey] !== input.action.ratio
    ) {
      nextGroup[billingKey] = input.action.ratio
      updated += 1
      continue
    }
    unchanged += 1
  }

  const ratios = { ...input.ratios }
  if (Object.keys(nextGroup).length > 0) {
    ratios[input.group] = nextGroup
  } else {
    delete ratios[input.group]
  }

  return {
    ratios,
    billingKeys,
    added,
    updated,
    removed,
    unchanged,
    collapsed: Math.max(0, input.modelNames.length - billingKeys.length),
  }
}
