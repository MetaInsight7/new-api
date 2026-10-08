export type CodexModelGovernanceStatus =
  | 'active'
  | 'unsupported_pending_review'
  | 'unsupported_disabled'
  | 'removed'
  | 'ignored'

export type CodexModelGovernanceReviewAction =
  | 'confirm_remove'
  | 'restore'
  | 'ignore'
  | 'disable'

export type CodexModelGovernanceRecord = {
  id: number
  model_name: string
  status: CodexModelGovernanceStatus
  source: string
  matched_rule: string
  last_error: string
  affected_channel_ids: number[]
  disabled_channel_ids: number[]
  abilities_disabled: boolean
  detected_at: number
  last_checked_at: number
  last_alerted_at: number
  reviewed_at: number
  reviewed_by: number
  review_note: string
}

export type CodexRuleTestResult = {
  matched: boolean
  model_name: string
  matched_rule: string
}
