import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { Badge } from '@/components/ui/badge'
import { formatTimestamp } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'

import {
  codexModelGovernanceQueryKeys,
  getCodexModelGovernanceRecords,
  reviewCodexModelGovernanceRecord,
} from './api'
import { GovernanceReviewActions } from './review-actions'
import type {
  CodexModelGovernanceRecord,
  CodexModelGovernanceReviewAction,
} from './types'

type ReviewSelection = {
  record: CodexModelGovernanceRecord
  action: CodexModelGovernanceReviewAction
  note: string
}

export function CodexModelGovernance() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [selection, setSelection] = useState<ReviewSelection | null>(null)
  const records = useQuery({
    queryKey: codexModelGovernanceQueryKeys.list(),
    queryFn: () => getCodexModelGovernanceRecords(),
  })
  const review = useMutation({
    mutationFn: (choice: ReviewSelection) =>
      reviewCodexModelGovernanceRecord(
        choice.record.id,
        choice.action,
        choice.note
      ),
    onSuccess: () => {
      toast.success(t('Review saved'))
      setSelection(null)
      void queryClient.invalidateQueries({
        queryKey: codexModelGovernanceQueryKeys.all,
      })
    },
    onError: (error) => handleServerError(error, t('Failed to save review')),
  })
  const statusLabel = (record: CodexModelGovernanceRecord): string => {
    switch (record.status) {
      case 'active':
        return t('Active')
      case 'unsupported_pending_review':
        return t('Pending review')
      case 'unsupported_disabled':
        return t('Routing disabled')
      case 'removed':
        return t('Removed')
      case 'ignored':
        return t('Ignored')
    }
  }
  const columns: StaticDataTableColumn<CodexModelGovernanceRecord>[] = [
    {
      id: 'model',
      header: t('Model'),
      cell: (record) => (
        <div className='max-w-lg min-w-48 space-y-1 break-words'>
          <div>{record.model_name}</div>
          <div className='text-muted-foreground text-xs'>{record.source}</div>
          <div className='text-xs break-all'>{record.matched_rule}</div>
          <div className='text-xs whitespace-pre-wrap'>{record.last_error}</div>
        </div>
      ),
    },
    {
      id: 'status',
      header: t('Status'),
      cell: (record) => (
        <div className='space-y-1'>
          <Badge variant='secondary'>{statusLabel(record)}</Badge>
          {(record.disabled_channel_ids?.length ?? 0) <
            (record.affected_channel_ids?.length ?? 0) &&
            record.status !== 'removed' && (
              <Badge variant='outline'>{t('Still serving')}</Badge>
            )}
        </div>
      ),
    },
    {
      id: 'channels',
      header: t('Affected channels'),
      cell: (record) => (
        <div className='min-w-36 text-xs'>
          <div>
            {(record.affected_channel_ids ?? [])
              .map((id) => `#${id}`)
              .join(', ') || '-'}
          </div>
          <div>
            {t('Routing disabled')}:{' '}
            {(record.disabled_channel_ids ?? [])
              .map((id) => `#${id}`)
              .join(', ') || '-'}
          </div>
        </div>
      ),
    },
    {
      id: 'time',
      header: t('Detected time'),
      cell: (record) => (
        <div className='min-w-40 space-y-1 text-xs'>
          <div>{formatTimestamp(record.detected_at)}</div>
          <div>
            {t('Last checked')}: {formatTimestamp(record.last_checked_at)}
          </div>
          <div>
            {t('Last alerted')}: {formatTimestamp(record.last_alerted_at)}
          </div>
          <div>
            {t('Reviewed by')}: {record.reviewed_by || '-'}
          </div>
          <div>{formatTimestamp(record.reviewed_at)}</div>
        </div>
      ),
    },
    {
      id: 'actions',
      header: t('Review note'),
      cell: (record) => (
        <GovernanceReviewActions
          record={record}
          pending={review.isPending}
          onReview={(action, note) => setSelection({ record, action, note })}
        />
      ),
    },
  ]
  let description = t('Save this review decision?')
  if (selection?.action === 'confirm_remove') {
    description = t(
      'Removing this model deletes it from the affected Codex channel configuration.'
    )
  }
  if (selection?.action === 'disable') {
    description = t(
      'Disabling this model stops routing on all affected Codex channels.'
    )
  }
  let content: ReactNode
  if (records.isError) {
    content = (
      <ErrorState
        title={t('Failed to load governance records')}
        onRetry={() => void records.refetch()}
      />
    )
  } else if (records.isPending) {
    content = <LoadingState />
  } else {
    content = (
      <StaticDataTable
        columns={columns}
        data={records.data ?? []}
        getRowKey={(record) => record.id}
        emptyContent={<EmptyState />}
      />
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Codex model governance')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        {content}
        <ConfirmDialog
          open={selection !== null}
          onOpenChange={(open) => {
            if (!open && !review.isPending) setSelection(null)
          }}
          title={t('Review model')}
          desc={description}
          confirmText={t('Confirm')}
          destructive={
            selection?.action === 'disable' ||
            selection?.action === 'confirm_remove'
          }
          isLoading={review.isPending}
          handleConfirm={() => {
            if (selection) review.mutate(selection)
          }}
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
