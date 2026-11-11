/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { AlertTriangle } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  Alert,
  AlertAction,
  AlertDescription,
  AlertTitle,
} from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

export function SensitiveWordMonitorSkeleton() {
  const { t } = useTranslation()
  return (
    <div className='flex flex-col gap-4' aria-label={t('Loading')}>
      <Skeleton className='h-48 w-full rounded-xl sm:h-40' />
      <div className='grid gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(18rem,1fr)]'>
        <Skeleton className='h-[28rem] w-full rounded-xl' />
        <Skeleton className='h-[28rem] w-full rounded-xl' />
      </div>
      <div className='grid gap-4 md:grid-cols-2 2xl:grid-cols-4'>
        {['rules', 'users', 'models', 'groups'].map((key) => (
          <Skeleton key={key} className='h-72 w-full rounded-xl' />
        ))}
      </div>
      <Skeleton className='h-80 w-full rounded-xl' />
    </div>
  )
}

export function SensitiveWordMonitorError(props: {
  isFetching: boolean
  onRetry: () => void
}) {
  const { t } = useTranslation()
  return (
    <Alert variant='destructive'>
      <AlertTriangle aria-hidden='true' />
      <AlertTitle>{t('Unable to load sensitive-word monitor')}</AlertTitle>
      <AlertDescription>
        {t('The operational dashboard could not be retrieved.')}
      </AlertDescription>
      <AlertAction>
        <Button
          size='sm'
          variant='outline'
          disabled={props.isFetching}
          onClick={props.onRetry}
        >
          {t('Retry')}
        </Button>
      </AlertAction>
    </Alert>
  )
}
