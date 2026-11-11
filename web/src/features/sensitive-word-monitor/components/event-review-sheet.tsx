/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ExternalLink, ShieldCheck, UserRoundCheck } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { SensitiveWordAuditSection } from '@/features/usage-logs/components/dialogs/sensitive-word-audit-section'
import {
  getUser,
  unbanSensitiveWordUser,
  updateSensitiveWordUserSafety,
} from '@/features/users/api'
import { USER_ROLE, USER_STATUS } from '@/features/users/constants'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { toIntlLocale } from '@/i18n/languages'
import {
  ADMIN_PERMISSION_ACTIONS,
  ADMIN_PERMISSION_RESOURCES,
  hasPermission,
} from '@/lib/admin-permissions'
import { formatNumber } from '@/lib/format'
import { handleServerError } from '@/lib/handle-server-error'
import {
  createServerError,
  requireServerSuccess,
} from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

import { sensitiveWordMonitorQueryKeys } from '../hooks/use-sensitive-word-monitor'
import type { SensitiveWordMonitorRecentEvent } from '../types'

type ReviewAction =
  | 'add_whitelist'
  | 'remove_whitelist'
  | 'clear_count'
  | 'unban'

export function SensitiveWordEventReviewSheet(props: {
  event: SensitiveWordMonitorRecentEvent | null
  onOpenChange: (open: boolean) => void
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const queryClient = useQueryClient()
  const currentUser = useAuthStore((state) => state.auth.user)
  const [pendingAction, setPendingAction] = useState<ReviewAction | null>(null)
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const event = props.event
  const canReadEvidence = hasPermission(
    currentUser,
    ADMIN_PERMISSION_RESOURCES.SENSITIVE_WORD,
    ADMIN_PERMISSION_ACTIONS.EVIDENCE_READ
  )
  const canManage = hasPermission(
    currentUser,
    ADMIN_PERMISSION_RESOURCES.SENSITIVE_WORD,
    ADMIN_PERMISSION_ACTIONS.MANAGE
  )

  const userQuery = useQuery({
    queryKey: [
      ...sensitiveWordMonitorQueryKeys.all,
      'user',
      event?.user_id ?? 0,
    ],
    queryFn: async () => {
      if (!event) throw new Error('Missing sensitive-word event')
      const response = requireServerSuccess(await getUser(event.user_id))
      if (!response.data) {
        throw createServerError(response, 'Unable to load affected user')
      }
      return response.data
    },
    enabled: Boolean(event && canManage),
    retry: false,
  })

  const actionMutation = useMutation({
    mutationFn: async (action: ReviewAction) => {
      if (!event) throw new Error('Missing sensitive-word event')
      if (action === 'unban') {
        requireServerSuccess(await unbanSensitiveWordUser(event.user_id))
        return
      }
      requireServerSuccess(
        await updateSensitiveWordUserSafety(
          event.user_id,
          action === 'clear_count'
            ? { violation_count: 0 }
            : { whitelist: action === 'add_whitelist' }
        )
      )
    },
    onSuccess: async (_result, action) => {
      if (action === 'add_whitelist') {
        toast.success(t('Added to sensitive-word whitelist'))
      } else if (action === 'remove_whitelist') {
        toast.success(t('Removed from sensitive-word whitelist'))
      } else if (action === 'clear_count') {
        toast.success(t('Sensitive-word violation count cleared'))
      } else {
        toast.success(t('Sensitive-word unban completed'))
      }
      setPendingAction(null)
      await queryClient.invalidateQueries({
        queryKey: sensitiveWordMonitorQueryKeys.all,
      })
    },
    onError: (error) => {
      handleServerError(error, t('Unable to update sensitive-word account'))
    },
  })

  const user = userQuery.data
  const canManageTarget = Boolean(
    user &&
    currentUser &&
    (currentUser.role === USER_ROLE.ROOT || currentUser.role > user.role)
  )
  const username = user?.username || event?.username || ''
  let confirmTitle = t('Sensitive-word unban')
  let confirmDescription = t(
    'Enable this account and clear its sensitive-word violation count? Historical audits, balance, subscriptions, API keys, and usage records stay unchanged.'
  )
  if (pendingAction === 'add_whitelist') {
    confirmTitle = t('Add to sensitive-word whitelist')
    confirmDescription = t(
      'Add {{username}} to the sensitive-word whitelist? Future matches will still be audited, but requests will be allowed and violations will not increase.',
      { username }
    )
  } else if (pendingAction === 'remove_whitelist') {
    confirmTitle = t('Remove from sensitive-word whitelist')
    confirmDescription = t(
      'Remove {{username}} from the sensitive-word whitelist? Future matches will follow the configured blocking and counting policy.',
      { username }
    )
  } else if (pendingAction === 'clear_count') {
    confirmTitle = t('Clear sensitive-word violation count')
    confirmDescription = t(
      'Clear the current sensitive-word violation count for {{username}}? Historical audit records remain unchanged.',
      { username }
    )
  }

  let accountContent: ReactNode = null
  if (userQuery.isLoading) {
    accountContent = (
      <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
    )
  } else if (userQuery.isError) {
    accountContent = (
      <Alert variant='destructive'>
        <AlertTitle>{t('Unable to load affected user')}</AlertTitle>
        <AlertDescription>
          {t('Refresh the panel and try again.')}
        </AlertDescription>
      </Alert>
    )
  } else if (user) {
    accountContent = (
      <>
        <div className='grid gap-3 rounded-lg border p-3 sm:grid-cols-3'>
          <div>
            <div className='text-muted-foreground text-xs'>{t('User')}</div>
            <div className='mt-1 truncate text-sm font-medium'>
              {user.username}
            </div>
          </div>
          <div>
            <div className='text-muted-foreground text-xs'>
              {t('Sensitive-word violations')}
            </div>
            <div className='mt-1 font-mono text-sm tabular-nums'>
              {formatNumber(user.sensitive_word_violation_count ?? 0, locale)}
            </div>
          </div>
          <div>
            <div className='text-muted-foreground text-xs'>
              {t('Current state')}
            </div>
            <div className='mt-1 flex flex-wrap gap-1.5'>
              <Badge
                variant={
                  user.status === USER_STATUS.DISABLED
                    ? 'destructive'
                    : 'secondary'
                }
              >
                {user.status === USER_STATUS.DISABLED
                  ? t('Disabled')
                  : t('Enabled')}
              </Badge>
              {user.sensitive_word_whitelist ? (
                <Badge variant='secondary'>
                  {t('Sensitive-word whitelist')}
                </Badge>
              ) : null}
            </div>
          </div>
        </div>
        {canManageTarget ? (
          <div className='flex flex-wrap gap-2'>
            <Button
              type='button'
              size='sm'
              variant='outline'
              onClick={() =>
                setPendingAction(
                  user.sensitive_word_whitelist
                    ? 'remove_whitelist'
                    : 'add_whitelist'
                )
              }
            >
              <ShieldCheck data-icon='inline-start' />
              {user.sensitive_word_whitelist
                ? t('Remove from whitelist')
                : t('Add to whitelist')}
            </Button>
            {(user.sensitive_word_violation_count ?? 0) > 0 ? (
              <Button
                type='button'
                size='sm'
                variant='outline'
                onClick={() => setPendingAction('clear_count')}
              >
                {t('Clear count')}
              </Button>
            ) : null}
            {(user.sensitive_word_violation_count ?? 0) > 0 ? (
              <Button
                type='button'
                size='sm'
                onClick={() => setPendingAction('unban')}
              >
                {t('Unban and clear count')}
              </Button>
            ) : null}
          </div>
        ) : (
          <p className='text-muted-foreground text-xs'>
            {t('Your role cannot modify this account.')}
          </p>
        )}
      </>
    )
  }

  return (
    <>
      <Sheet
        open={Boolean(event)}
        onOpenChange={(open) => {
          if (!open) setPendingAction(null)
          props.onOpenChange(open)
        }}
      >
        {event ? (
          <SheetContent className={sideDrawerContentClassName('sm:max-w-3xl')}>
            <SheetHeader className={sideDrawerHeaderClassName()}>
              <SheetTitle>{t('Review sensitive-word event')}</SheetTitle>
              <SheetDescription>
                {t(
                  'Review retained request context and handle the affected account without leaving this page.'
                )}
              </SheetDescription>
            </SheetHeader>

            <div className='flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto overscroll-contain px-4 py-4 sm:px-6 sm:py-5'>
              <SideDrawerSection>
                <SideDrawerSectionHeader
                  title={t('Account handling')}
                  description={t(
                    'These actions update the current account state; historical audit records remain unchanged.'
                  )}
                  icon={<UserRoundCheck />}
                  iconTone='warning'
                />
                {accountContent}
              </SideDrawerSection>

              <SideDrawerSection>
                <SideDrawerSectionHeader
                  title={t('Retained request context')}
                  description={t(
                    'Shows system and developer context, the matched round, and up to three preceding rounds when retained.'
                  )}
                  icon={<ShieldCheck />}
                  iconTone='primary'
                />
                {canReadEvidence && event.audit_id ? (
                  <SensitiveWordAuditSection
                    auditId={event.audit_id}
                    log={{
                      request_id: event.request_id,
                      group: event.group_name,
                      model_name: event.model_name,
                    }}
                    other={null}
                    isAdmin
                    open
                    copiedText={copiedText}
                    onCopy={copyToClipboard}
                  />
                ) : (
                  <Alert>
                    <AlertTitle>
                      {t('No permission to view this audit evidence')}
                    </AlertTitle>
                    <AlertDescription>
                      {t(
                        'Ask a Root administrator to grant the sensitive-word evidence-read permission.'
                      )}
                    </AlertDescription>
                  </Alert>
                )}
              </SideDrawerSection>
            </div>

            <SheetFooter className={sideDrawerFooterClassName()}>
              <SheetClose render={<Button variant='outline' />}>
                {t('Close')}
              </SheetClose>
              {event.request_id ? (
                <Button
                  variant='outline'
                  render={
                    <Link
                      to='/usage-logs/$section'
                      params={{ section: 'common' }}
                      search={{ type: ['8'], requestId: event.request_id }}
                    />
                  }
                >
                  {t('View full log')}
                  <ExternalLink data-icon='inline-end' />
                </Button>
              ) : null}
            </SheetFooter>
          </SheetContent>
        ) : null}
      </Sheet>

      <ConfirmDialog
        open={pendingAction !== null}
        onOpenChange={(open) => {
          if (!open) setPendingAction(null)
        }}
        title={confirmTitle}
        desc={confirmDescription}
        confirmText={
          pendingAction === 'unban' ? t('Unban and clear count') : t('Confirm')
        }
        handleConfirm={() => {
          if (pendingAction) actionMutation.mutate(pendingAction)
        }}
        isLoading={actionMutation.isPending}
      />
    </>
  )
}
