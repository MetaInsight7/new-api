import { ArrowRight01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { cn } from '@/lib/utils'

import type { ModelAccessScope } from '../types'

type ModelAccessScopeRailProps = {
  scopes: ModelAccessScope[]
  modelCounts: ReadonlyMap<string, number>
  selectedScopeId: string | null
  onScopeChange: (scopeId: string) => void
}

export function ModelAccessScopeRail({
  scopes,
  modelCounts,
  selectedScopeId,
  onScopeChange,
}: ModelAccessScopeRailProps) {
  const { t } = useTranslation()

  return (
    <div className='flex flex-col'>
      <RadioGroup
        className='gap-2'
        value={selectedScopeId ?? undefined}
        aria-label={t('Access groups')}
        onValueChange={onScopeChange}
      >
        {scopes.map((scope, index) => {
          const selected = scope.id === selectedScopeId
          const inputId = `model-access-scope-${index}`
          return (
            <div
              key={scope.id}
              className={cn(
                'bg-card text-card-foreground border-border relative overflow-hidden rounded-lg border text-sm transition-colors',
                'hover:border-primary/40',
                selected && 'border-primary bg-primary/5 ring-primary/20 ring-1'
              )}
            >
              <Label
                htmlFor={inputId}
                className='flex h-14 cursor-pointer items-center justify-between gap-3 px-3 text-left'
              >
                <span className='min-w-0'>
                  <span className='block truncate text-sm leading-5 font-semibold'>
                    {scope.label}
                  </span>
                  <span className='text-muted-foreground mt-0.5 flex min-w-0 items-center gap-1.5 text-[11px] leading-4'>
                    <span className='truncate'>
                      {t('Available Models')}{' '}
                      <strong className='text-foreground font-semibold tabular-nums'>
                        {modelCounts.get(scope.id) ?? 0}
                      </strong>
                    </span>
                    <span aria-hidden='true'>·</span>
                    <span className='truncate'>
                      {t('Group Ratio')}{' '}
                      <strong className='text-foreground font-semibold tabular-nums'>
                        {scope.ratio === null ? '—' : `${scope.ratio}×`}
                      </strong>
                    </span>
                  </span>
                </span>
                <RadioGroupItem
                  id={inputId}
                  value={scope.id}
                  aria-label={scope.label}
                  className='shrink-0'
                />
              </Label>
              {selected && (
                <Button
                  size='sm'
                  variant='ghost'
                  className='text-primary hover:bg-background/80 hover:text-primary border-primary/20 h-[34px] w-full justify-center rounded-none border-t px-3 text-xs font-medium'
                  render={<Link to='/keys' />}
                >
                  {t('Use this group to create an API key')}
                  <HugeiconsIcon
                    icon={ArrowRight01Icon}
                    strokeWidth={2}
                    data-icon='inline-end'
                    aria-hidden='true'
                  />
                </Button>
              )}
            </div>
          )
        })}
      </RadioGroup>
    </div>
  )
}
