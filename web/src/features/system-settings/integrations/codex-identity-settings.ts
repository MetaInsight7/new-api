import { formatDateTimeStr } from '@/lib/format'

import type { CodexIdentityOptionUpdate } from '../types'

export type CodexIdentityFormValues = {
  CodexClientUserAgent: string
  CodexClientVersion: string
  CodexSyncedClientVersion: string
  CodexSyncedClientVersionAt: string
  CodexAutoSyncClientVersion: boolean
  CodexEnforceClientIdentity: boolean
}

export type { CodexIdentityOptionUpdate }

export function buildCodexIdentityOptionUpdates(
  defaults: CodexIdentityFormValues,
  values: CodexIdentityFormValues
): CodexIdentityOptionUpdate[] {
  const updates: CodexIdentityOptionUpdate[] = []
  const nextUserAgent = values.CodexClientUserAgent.trim()
  const nextVersion = values.CodexClientVersion.trim()

  if (nextUserAgent !== defaults.CodexClientUserAgent.trim()) {
    updates.push({ key: 'CodexClientUserAgent', value: nextUserAgent })
  }
  if (nextVersion !== defaults.CodexClientVersion.trim()) {
    updates.push({ key: 'CodexClientVersion', value: nextVersion })
  }
  if (
    values.CodexAutoSyncClientVersion !== defaults.CodexAutoSyncClientVersion
  ) {
    updates.push({
      key: 'CodexAutoSyncClientVersion',
      value: String(values.CodexAutoSyncClientVersion),
    })
  }
  if (
    values.CodexEnforceClientIdentity !== defaults.CodexEnforceClientIdentity
  ) {
    updates.push({
      key: 'CodexEnforceClientIdentity',
      value: String(values.CodexEnforceClientIdentity),
    })
  }
  return updates
}

export function formatSyncTime(value: string): string {
  const raw = value.trim()
  if (raw === '') return ''

  const seconds = Number(raw)
  if (Number.isFinite(seconds) && seconds > 0) {
    return formatDateTimeStr(new Date(seconds * 1000))
  }

  const parsed = Date.parse(raw)
  if (!Number.isFinite(parsed)) return ''
  return formatDateTimeStr(new Date(parsed))
}
