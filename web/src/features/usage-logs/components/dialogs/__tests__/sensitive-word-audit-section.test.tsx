/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import type { UsageLog } from '../../../data/schema'
import { DetailsDialog } from '../details-dialog'
import { SensitiveWordAuditSection } from '../sensitive-word-audit-section'

const log: UsageLog = {
  id: 10,
  user_id: 20,
  created_at: 1,
  type: 8,
  content: '',
  username: 'subject',
  token_name: 'key',
  model_name: 'model',
  quota: 0,
  prompt_tokens: 0,
  completion_tokens: 0,
  use_time: 0,
  is_stream: false,
  channel: 0,
  channel_name: '',
  token_id: 2,
  group: 'default',
  ip: '',
  other: '',
  request_id: 'request-1',
  upstream_request_id: '',
}

function renderSection() {
  render(
    <SensitiveWordAuditSection
      auditId={12}
      log={log}
      other={null}
      isAdmin
      open
      copiedText={null}
      onCopy={() => undefined}
    />
  )
}

afterEach(() => {
  useAuthStore.getState().auth.reset()
  vi.restoreAllMocks()
})

it('does not request or render evidence without evidence-read permission', async () => {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'manager',
    role: 10,
    permissions: {
      admin_permissions: {
        sensitive_word: { manage: true, evidence_read: false },
      },
    },
  })
  const get = vi.spyOn(api, 'get')

  renderSection()

  await waitFor(() => expect(get).not.toHaveBeenCalled())
  expect(screen.queryByText('Sensitive word audit')).not.toBeInTheDocument()
})

it('loads and renders bounded structured evidence for an authorized reader', async () => {
  useAuthStore.getState().auth.setUser({
    id: 2,
    username: 'auditor',
    role: 10,
    permissions: {
      admin_permissions: {
        sensitive_word: { manage: false, evidence_read: true },
      },
    },
  })
  const get = vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        id: 12,
        request_id: 'request-1',
        user_id: 20,
        username_snapshot: 'subject',
        blocked: true,
        violation_count: 3,
        evidence_truncated: true,
        matched_words: '["blocked-word"]',
        matched_rule_names: '["production rule"]',
        matched_rule_ids: '[9]',
        evidence: JSON.stringify({
          version: 1,
          truncated: true,
          matches: [],
          fields: [
            {
              role: 'system',
              kind: 'content',
              round: -1,
              order: 0,
              text: 'system context',
            },
            {
              role: 'user',
              kind: 'content',
              round: 2,
              order: 1,
              text: 'bounded user evidence',
            },
          ],
        }),
      },
    },
  })

  renderSection()

  expect(await screen.findByText('bounded user evidence')).toBeVisible()
  expect(screen.getByText('system context')).toBeVisible()
  expect(screen.getByText('blocked-word')).toBeVisible()
  expect(screen.getByText('Round 3')).toBeVisible()
  expect(get).toHaveBeenCalledWith('/api/log/sensitive-word-audit/12')
})

it('shows an evidence loading failure without exposing fallback details', async () => {
  useAuthStore.getState().auth.setUser({
    id: 3,
    username: 'auditor',
    role: 10,
    permissions: {
      admin_permissions: {
        sensitive_word: { evidence_read: true },
      },
    },
  })
  vi.spyOn(api, 'get').mockRejectedValue(new Error('forbidden'))

  renderSection()

  expect(await screen.findByText('Unable to load evidence')).toBeVisible()
  expect(screen.queryByText('bounded user evidence')).not.toBeInTheDocument()
})

it('loads linked evidence from the usage-log admin projection', async () => {
  useAuthStore.getState().auth.setUser({
    id: 4,
    username: 'auditor',
    role: 10,
    permissions: {
      admin_permissions: {
        sensitive_word: { evidence_read: true },
      },
    },
  })
  const get = vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        id: 44,
        request_id: 'request-1',
        user_id: 20,
        evidence: JSON.stringify({
          version: 1,
          truncated: false,
          matches: [],
          fields: [
            {
              role: 'user',
              kind: 'content',
              round: 0,
              order: 0,
              text: 'linked audit evidence',
            },
          ],
        }),
      },
    },
  })
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const freshAt = Date.now() + 60_000
  queryClient.setQueryData(['status'], {}, { updatedAt: freshAt })
  queryClient.setQueryData(
    ['pricing'],
    { data: [], vendors: [] },
    { updatedAt: freshAt }
  )

  render(
    <QueryClientProvider client={queryClient}>
      <DetailsDialog
        log={{
          ...log,
          other: JSON.stringify({
            action: 'sensitive_word_block',
            admin_info: { keyword_filter: { audit_id: 44 } },
          }),
        }}
        isAdmin
        isRoot={false}
        open
        onOpenChange={() => undefined}
      />
    </QueryClientProvider>
  )

  expect(await screen.findByText('linked audit evidence')).toBeVisible()
  expect(get).toHaveBeenCalledWith('/api/log/sensitive-word-audit/44')
  queryClient.clear()
})
