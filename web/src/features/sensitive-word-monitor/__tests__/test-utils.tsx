/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { render } from '@testing-library/react'

import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { SensitiveWordMonitor } from '..'
import type {
  SensitiveWordMonitorDashboard,
  SensitiveWordMonitorWindow,
} from '../types'

export function sensitiveWordDashboard(
  hours: SensitiveWordMonitorWindow = 24
): SensitiveWordMonitorDashboard {
  return {
    requested_hours: hours,
    sensitive_metadata_included: true,
    bucket_seconds: hours === 24 ? 3600 : 86400,
    window_start: 1_800_000_000,
    generated_at: 1_800_003_600,
    timezone: 'Asia/Shanghai',
    policy: {
      enabled: true,
      check_prompt: true,
      auto_ban_enabled: true,
      ban_threshold: 50,
      full_prompt_retention_days: 180,
      rule_count: 3,
      blocking_rule_count: 1,
      observe_rule_count: 1,
      disabled_rule_count: 1,
    },
    summary: {
      matched_requests: hours,
      blocked_requests: 4,
      observed_requests: 3,
      whitelist_bypasses: 2,
      affected_users: 5,
      automatic_bans: 1,
    },
    series: [
      {
        timestamp: 1_800_000_000,
        matched_requests: hours,
        blocked_requests: 4,
        observed_requests: 3,
        whitelist_bypasses: 2,
        automatic_bans: 1,
      },
    ],
    top_rules: [
      {
        rule_id: 1,
        rule_name: 'Credential abuse',
        match_count: 9,
        blocked_count: 4,
      },
    ],
    top_users: [
      {
        user_id: 7,
        username: 'alice',
        match_count: 9,
        blocked_count: 4,
        automatic_bans: 1,
        last_seen_at: 1_800_003_600,
      },
    ],
    top_models: [{ name: 'gpt-test', count: 9 }],
    top_groups: [{ name: 'default', count: 9 }],
    recent_events: [
      {
        audit_id: 10,
        request_id: 'request-sensitive-10',
        user_id: 7,
        username: 'alice',
        model_name: 'gpt-test',
        group_name: 'default',
        disposition: 'block',
        rule_names: ['Credential abuse'],
        violation_count: 3,
        automatic_ban: false,
        created_at: 1_800_003_600,
      },
    ],
  }
}

export function affectedSensitiveWordUser() {
  return {
    id: 7,
    username: 'alice',
    role: ROLE.USER,
    status: 1,
    sensitive_word_violation_count: 3,
    sensitive_word_whitelist: false,
  }
}

export function setSensitiveWordManager(options?: { evidenceRead?: boolean }) {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'manager',
    role: ROLE.ADMIN,
    permissions: {
      admin_permissions: {
        sensitive_word: {
          manage: true,
          evidence_read: options?.evidenceRead ?? true,
        },
      },
    },
  })
}

export async function renderSensitiveWordMonitor() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const root = createRootRoute()
  const authenticated = createRoute({
    getParentRoute: () => root,
    id: '_authenticated',
  })
  const monitor = createRoute({
    getParentRoute: () => authenticated,
    path: 'sensitive-word-monitor/',
    component: SensitiveWordMonitor,
  })
  const logs = createRoute({
    getParentRoute: () => authenticated,
    path: 'usage-logs/$section',
    component: () => null,
  })
  const router = createRouter({
    routeTree: root.addChildren([authenticated.addChildren([monitor, logs])]),
    history: createMemoryHistory({
      initialEntries: ['/sensitive-word-monitor/'],
    }),
  })

  await router.load()
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}
