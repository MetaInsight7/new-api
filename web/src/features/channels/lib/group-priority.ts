export type GroupPriorityOverride = {
  priority?: number
  weight?: number
}

export type GroupPriorityOverrides = Record<string, GroupPriorityOverride>
export type GroupPriorityDocument = Record<string, unknown>

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function parseGroupPriorityDocument(
  raw: string | null | undefined
): GroupPriorityDocument | null {
  if (!raw?.trim()) return {}

  try {
    const parsed: unknown = JSON.parse(raw)
    return isRecord(parsed) ? parsed : null
  } catch {
    return null
  }
}

export function parseGroupPriority(
  raw: string | null | undefined
): GroupPriorityOverrides {
  const parsed = parseGroupPriorityDocument(raw)
  if (parsed === null) return {}

  return Object.fromEntries(
    Object.entries(parsed).flatMap(([group, value]) => {
      if (!group.trim() || !isRecord(value)) return []

      const override: GroupPriorityOverride = {}
      if (
        typeof value.priority === 'number' &&
        Number.isFinite(value.priority)
      ) {
        override.priority = value.priority
      }
      if (
        typeof value.weight === 'number' &&
        Number.isInteger(value.weight) &&
        value.weight >= 0
      ) {
        override.weight = value.weight
      }

      return Object.keys(override).length > 0 ? [[group, override]] : []
    })
  )
}

export function setGroupPriorityOverride(
  raw: string | null | undefined,
  group: string,
  override: GroupPriorityOverride | null
): string | null {
  const parsed = parseGroupPriorityDocument(raw)
  if (parsed === null) return null

  if (override === null) {
    const next = { ...parsed }
    delete next[group]
    return JSON.stringify(next)
  }

  const stored = Object.hasOwn(parsed, group) ? parsed[group] : undefined
  const current = isRecord(stored) ? stored : {}
  const next = { ...parsed, [group]: { ...current, ...override } }
  return JSON.stringify(next)
}
