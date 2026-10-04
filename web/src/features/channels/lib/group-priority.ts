export type GroupPriorityOverride = {
  priority?: number
  weight?: number
}

export type GroupPriorityOverrides = Record<string, GroupPriorityOverride>

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function parseGroupPriority(
  raw: string | null | undefined
): GroupPriorityOverrides {
  if (!raw?.trim()) return {}

  try {
    const parsed: unknown = JSON.parse(raw)
    if (!isRecord(parsed)) return {}

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
  } catch {
    return {}
  }
}
