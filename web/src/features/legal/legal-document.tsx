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
import { useQuery } from '@tanstack/react-query'
import { ExternalLink, FileWarning, Scale } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { RichContent } from '@/components/rich-content'
import { Skeleton } from '@/components/ui/skeleton'
import { AuthNavbar } from '@/features/auth/auth-navbar'
import { isHttpUrl, isLikelyHtml } from '@/lib/content-format'

import type { LegalDocumentResponse } from './types'

type LegalDocumentProps = {
  title: string
  queryKey: string
  fetchDocument: () => Promise<LegalDocumentResponse>
  emptyMessage: string
}

export function LegalDocument({
  title,
  queryKey,
  fetchDocument,
  emptyMessage,
}: LegalDocumentProps) {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: [queryKey],
    queryFn: fetchDocument,
    staleTime: 10 * 60 * 1000,
  })

  const rawContent = data?.data?.trim() ?? ''
  const hasContent = rawContent.length > 0
  const isUrl = hasContent && isHttpUrl(rawContent)
  const contentIsHtml = hasContent && isLikelyHtml(rawContent)
  const success = data?.success ?? false
  const titleId = `${queryKey}-title`
  let documentContent: ReactNode

  if (isLoading) {
    documentContent = (
      <div className='legal-document-loading' aria-label={t('Loading')}>
        <Skeleton className='h-4 w-full' />
        <Skeleton className='h-4 w-[92%]' />
        <Skeleton className='h-4 w-[84%]' />
        <Skeleton className='h-4 w-[88%]' />
      </div>
    )
  } else if (!success || !hasContent) {
    documentContent = (
      <div className='legal-document-state' role='status'>
        <span className='legal-document-state-icon' aria-hidden='true'>
          <FileWarning />
        </span>
        <div>
          <p>{data?.message || emptyMessage}</p>
        </div>
      </div>
    )
  } else if (isUrl) {
    documentContent = (
      <div className='legal-document-state'>
        <p>
          {t(
            'The administrator configured an external link for this document.'
          )}
        </p>
        <a
          className='legal-document-action'
          href={rawContent}
          target='_blank'
          rel='noopener noreferrer'
        >
          {t('View document')}
          <ExternalLink aria-hidden='true' />
        </a>
      </div>
    )
  } else {
    documentContent = (
      <RichContent
        mode={contentIsHtml ? 'html' : 'markdown'}
        htmlVariant={contentIsHtml ? 'isolated' : undefined}
        content={rawContent}
        className='legal-document-content prose-neutral dark:prose-invert max-w-none'
      />
    )
  }

  return (
    <div className='auth-page legal-page'>
      <AuthNavbar />
      <main className='legal-main'>
        <article
          className='legal-document-card'
          aria-labelledby={titleId}
          aria-busy={isLoading}
        >
          <div className='legal-document-header'>
            <span className='legal-document-mark' aria-hidden='true'>
              <Scale />
            </span>
            <div>
              <span className='legal-document-brand'>Token Dazi</span>
              <h1 id={titleId}>{title}</h1>
            </div>
          </div>
          <div className='legal-document-body'>{documentContent}</div>
        </article>
      </main>
    </div>
  )
}
