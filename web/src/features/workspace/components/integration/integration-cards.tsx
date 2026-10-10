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
import { ArrowRight, Braces, Bot, TerminalSquare } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  CardStaggerContainer,
  CardStaggerItem,
} from '@/components/page-transition'

import type { IntegrationId } from './integration-snippets'

interface IntegrationCard {
  id: IntegrationId
  title: string
  description: string
  icon: typeof Braces
  tone: 'neutral' | 'blue' | 'amber' | 'teal'
  badge?: string
}

function getIconTone(tone: IntegrationCard['tone']): string {
  if (tone === 'blue') {
    return 'bg-[#e6eeff] text-[#386fe5] dark:bg-violet-900/45 dark:text-violet-200'
  }
  if (tone === 'amber') {
    return 'bg-[#fff2dc] text-[#b87921] dark:bg-amber-900/35 dark:text-amber-200'
  }
  if (tone === 'teal') {
    return 'bg-[#d8f5ec] text-[#1d9d79] dark:bg-emerald-900/35 dark:text-emerald-200'
  }
  return 'bg-[#f4f1f8] text-foreground dark:bg-violet-900/35 dark:text-violet-100'
}

// eslint-disable-next-line react-refresh/only-export-components
export function useIntegrationCards(): IntegrationCard[] {
  const { t } = useTranslation()

  return [
    {
      id: 'api',
      title: t('API for developers'),
      description: t(
        'Call any model with an OpenAI-compatible API. Copy a ready-to-run example for your model and language.'
      ),
      icon: TerminalSquare,
      tone: 'neutral',
    },
    {
      id: 'sdk',
      title: t('SDKs for developers'),
      description: t(
        'Use the OpenAI SDK you already know — with TokenDazi as the gateway.'
      ),
      icon: Braces,
      tone: 'blue',
    },
    {
      id: 'agent',
      title: t('Codex & Claude Code'),
      description: t(
        'Connect Codex or Claude Code with one command, then use TokenDazi from your existing projects.'
      ),
      icon: Bot,
      tone: 'teal',
    },
  ]
}

export function IntegrationCards(props: {
  onSelect: (id: IntegrationId) => void
}) {
  const { t } = useTranslation()
  const cards = useIntegrationCards()

  return (
    <section className='bg-card overflow-hidden rounded-2xl border shadow-xs'>
      <div className='border-b px-4 py-3 sm:px-5'>
        <h2 className='text-sm font-semibold'>
          {t('Choose how you will use TokenDazi')}
        </h2>
        <p className='text-muted-foreground mt-1 text-xs leading-4'>
          {t('All three options use the same account and model catalog.')}
        </p>
      </div>

      <CardStaggerContainer className='divide-y'>
        {cards.map((card) => {
          const Icon = card.icon

          return (
            <CardStaggerItem key={card.id}>
              <button
                type='button'
                onClick={() => props.onSelect(card.id)}
                className='group focus-visible:ring-ring hover:bg-muted/50 flex w-full items-center gap-3 px-4 py-3 text-left transition-colors focus-visible:ring-2 focus-visible:outline-none sm:px-5'
              >
                <span
                  className={`flex size-8 shrink-0 items-center justify-center rounded-lg ${getIconTone(card.tone)}`}
                >
                  <Icon className='size-4' aria-hidden='true' />
                </span>
                <span className='min-w-0 flex-1'>
                  <span className='flex items-center gap-2 text-sm font-semibold'>
                    {card.title}
                    {card.badge && (
                      <span className='bg-muted text-muted-foreground rounded-md px-1.5 py-0.5 text-[10px] font-medium'>
                        {card.badge}
                      </span>
                    )}
                  </span>
                  <span className='text-muted-foreground mt-0.5 block text-xs leading-4'>
                    {card.description}
                  </span>
                </span>
                <ArrowRight
                  className='text-muted-foreground group-hover:text-foreground size-4 shrink-0 transition-colors'
                  aria-hidden='true'
                />
              </button>
            </CardStaggerItem>
          )
        })}
      </CardStaggerContainer>
    </section>
  )
}
