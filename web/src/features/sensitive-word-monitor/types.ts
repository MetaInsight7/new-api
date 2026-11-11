/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/

export type SensitiveWordMonitorWindow = 24 | 168 | 720

export type SensitiveWordMonitorPolicy = {
  enabled: boolean
  check_prompt: boolean
  auto_ban_enabled: boolean
  ban_threshold: number
  full_prompt_retention_days: number
  rule_count: number
  blocking_rule_count: number
  observe_rule_count: number
  disabled_rule_count: number
}

export type SensitiveWordMonitorSummary = {
  matched_requests: number
  blocked_requests: number
  observed_requests: number
  whitelist_bypasses: number
  affected_users: number
  automatic_bans: number
}

export type SensitiveWordMonitorSeriesPoint = {
  timestamp: number
  matched_requests: number
  blocked_requests: number
  observed_requests: number
  whitelist_bypasses: number
  automatic_bans: number
}

export type SensitiveWordMonitorTopRule = {
  rule_id: number
  rule_name: string
  match_count: number
  blocked_count: number
}

export type SensitiveWordMonitorTopUser = {
  user_id: number
  username: string
  match_count: number
  blocked_count: number
  automatic_bans: number
  last_seen_at: number
}

export type SensitiveWordMonitorDimension = {
  name: string
  count: number
}

export type SensitiveWordMonitorDisposition =
  | 'block'
  | 'observe'
  | 'whitelist_bypass'

export type SensitiveWordMonitorRecentEvent = {
  audit_id?: number
  request_id: string
  user_id: number
  username: string
  model_name: string
  group_name: string
  disposition: SensitiveWordMonitorDisposition
  rule_names?: string[]
  violation_count: number
  automatic_ban: boolean
  created_at: number
}

export type SensitiveWordMonitorDashboard = {
  requested_hours: SensitiveWordMonitorWindow
  sensitive_metadata_included: boolean
  bucket_seconds: number
  window_start: number
  generated_at: number
  timezone: string
  policy: SensitiveWordMonitorPolicy
  summary: SensitiveWordMonitorSummary
  series: SensitiveWordMonitorSeriesPoint[]
  top_rules: SensitiveWordMonitorTopRule[]
  top_users: SensitiveWordMonitorTopUser[]
  top_models: SensitiveWordMonitorDimension[]
  top_groups: SensitiveWordMonitorDimension[]
  recent_events: SensitiveWordMonitorRecentEvent[]
}

export type ApiEnvelope<T> = {
  success: boolean
  message?: string
  data: T
}
