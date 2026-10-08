import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormMessage,
} from '@/components/ui/form'
import { Textarea } from '@/components/ui/textarea'
import { hasPermission } from '@/lib/admin-permissions'
import { useAuthStore } from '@/stores/auth-store'

import {
  governanceReviewNoteSchema,
  type GovernanceReviewNoteFormValues,
} from './lib/review-note'
import type {
  CodexModelGovernanceRecord,
  CodexModelGovernanceReviewAction,
} from './types'

export function GovernanceReviewActions(props: {
  record: CodexModelGovernanceRecord
  onReview: (action: CodexModelGovernanceReviewAction, note: string) => void
  pending: boolean
}) {
  const { t } = useTranslation()
  const form = useForm<GovernanceReviewNoteFormValues>({
    resolver: zodResolver(governanceReviewNoteSchema),
    values: { note: props.record.review_note ?? '' },
  })
  const user = useAuthStore((state) => state.auth.user)
  const canOperate = hasPermission(user, 'channel', 'operate')
  const canWrite = hasPermission(user, 'channel', 'write')
  const disabled = props.record.disabled_channel_ids ?? []
  const affected = props.record.affected_channel_ids ?? []
  const blocked = props.record.abilities_disabled || disabled.length > 0
  const requestReview = (action: CodexModelGovernanceReviewAction) => {
    void form.handleSubmit((values) => props.onReview(action, values.note))()
  }

  return (
    <Form {...form}>
      <div className='min-w-60 space-y-2'>
        <FormField
          control={form.control}
          name='note'
          render={({ field }) => (
            <FormItem>
              <FormControl>
                <Textarea
                  {...field}
                  aria-label={t('Review note')}
                  disabled={props.pending || (!canOperate && !canWrite)}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <div className='flex flex-wrap gap-2'>
          {canOperate &&
            disabled.length < affected.length &&
            props.record.status !== 'removed' && (
              <Button
                size='sm'
                variant='destructive'
                disabled={props.pending}
                onClick={() => requestReview('disable')}
              >
                {t('Disable model')}
              </Button>
            )}
          {canWrite && props.record.status !== 'removed' && (
            <Button
              size='sm'
              variant='destructive'
              disabled={props.pending}
              onClick={() => requestReview('confirm_remove')}
            >
              {t('Confirm removal')}
            </Button>
          )}
          {canOperate && props.record.status !== 'removed' && (
            <Button
              size='sm'
              variant='outline'
              disabled={props.pending}
              onClick={() => requestReview('restore')}
            >
              {t('Restore model')}
            </Button>
          )}
          {canOperate && !blocked && props.record.status !== 'removed' && (
            <Button
              size='sm'
              variant='outline'
              disabled={props.pending}
              onClick={() => requestReview('ignore')}
            >
              {t('Ignore finding')}
            </Button>
          )}
        </div>
      </div>
    </Form>
  )
}
