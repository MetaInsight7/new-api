import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import type {
  CodexModelGovernanceRecord,
  CodexModelGovernanceReviewAction,
  CodexModelGovernanceStatus,
  CodexRuleTestResult,
} from './types'

type Envelope<T> = { success: boolean; message?: string; data: T }

export const codexModelGovernanceQueryKeys = {
  all: ['codex-model-governance'] as const,
  list: (status?: CodexModelGovernanceStatus) =>
    ['codex-model-governance', 'list', status] as const,
}

export async function getCodexModelGovernanceRecords(
  status?: CodexModelGovernanceStatus
): Promise<CodexModelGovernanceRecord[]> {
  const response = await api.get<Envelope<CodexModelGovernanceRecord[]>>(
    '/api/codex_model_governance/',
    { params: { status } }
  )
  return requireServerSuccess(response.data).data
}

export async function reviewCodexModelGovernanceRecord(
  id: number,
  action: CodexModelGovernanceReviewAction,
  note: string
): Promise<void> {
  const response = await api.post(
    `/api/codex_model_governance/${id}/review/${action}`,
    {
      action,
      note,
    }
  )
  requireServerSuccess(response.data)
}

export async function testCodexModelGovernanceRule(
  message: string,
  patterns: string[]
): Promise<CodexRuleTestResult> {
  const response = await api.post<Envelope<CodexRuleTestResult>>(
    '/api/codex_model_governance/rules/test',
    { message, patterns }
  )
  return requireServerSuccess(response.data).data
}

export async function triggerCodexModelGovernanceTask(
  source: 'probe' | 'notice'
): Promise<{ task_id: string; created: boolean }> {
  const response = await api.post<
    Envelope<{ task_id: string; created: boolean }>
  >('/api/codex_model_governance/trigger', null, { params: { source } })
  return requireServerSuccess(response.data).data
}
