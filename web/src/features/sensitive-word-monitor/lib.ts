/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import type {
  SensitiveWordMonitorDisposition,
  SensitiveWordMonitorWindow,
} from './types'

type SensitiveWordTooltipPayload = {
  payload?: {
    timestamp?: unknown
  }
}

export function getSensitiveWordTooltipTimestamp(
  payload: readonly SensitiveWordTooltipPayload[] | undefined
): number | null {
  const value = payload?.[0]?.payload?.timestamp
  if (typeof value !== 'number' && typeof value !== 'string') return null

  const timestamp = Number(value)
  return Number.isFinite(timestamp) ? timestamp : null
}

export function sensitiveWordMonitorWindowLabel(
  hours: SensitiveWordMonitorWindow
): string {
  if (hours === 24) return '24h'
  if (hours === 168) return '7d'
  return '30d'
}

export function sensitiveWordDispositionLabel(
  disposition: SensitiveWordMonitorDisposition
): string {
  if (disposition === 'block') return 'Blocked'
  if (disposition === 'whitelist_bypass') return 'Whitelist bypassed'
  return 'Observed'
}
