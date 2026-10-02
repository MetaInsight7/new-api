import {
  Alert02Icon,
  Copy01Icon,
  PackageIcon,
} from '@hugeicons/core-free-icons'
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
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemFooter,
  ItemGroup,
  ItemMedia,
  ItemTitle,
} from '@/components/ui/item'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getLobeIcon } from '@/lib/lobe-icon'
import { getModelAvailabilityConfig } from '@/lib/model-availability'
import { cn } from '@/lib/utils'
import {
  DEFAULT_TOKEN_UNIT,
  QUOTA_TYPE_VALUES,
} from '@/features/pricing/constants'
import {
  formatFixedPrice,
  formatGroupPrice,
} from '@/features/pricing/lib/price'
import type { PricingModel } from '@/features/pricing/types'

import {
  getModelEndpointLabel,
  normalizeModelAvailabilityStatus,
} from '../lib/model-access-browser'
import type { ModelAccessModel } from '../types'

type ModelAccessListProps = {
  pricingModels?: ReadonlyMap<string, PricingModel>
  pricingLoading?: boolean
  models: ModelAccessModel[]
  scopeIsEmpty: boolean
  onClearFilters: () => void
}

export const MODEL_ACCESS_PAGE_SIZE = 50

type ModelAccessPaginationState = {
  models: ModelAccessModel[]
  scopeIsEmpty: boolean
  visibleCount: number
}

// eslint-disable-next-line react-refresh/only-export-components
export function getNextVisibleModelCount(
  currentCount: number,
  totalCount: number
): number {
  return Math.min(currentCount + MODEL_ACCESS_PAGE_SIZE, totalCount)
}

// eslint-disable-next-line react-refresh/only-export-components
export function getEffectiveVisibleModelCount(
  pagination: ModelAccessPaginationState,
  models: ModelAccessModel[],
  scopeIsEmpty: boolean
): number {
  if (
    pagination.models !== models ||
    pagination.scopeIsEmpty !== scopeIsEmpty
  ) {
    return MODEL_ACCESS_PAGE_SIZE
  }
  return pagination.visibleCount
}

export function ModelAccessList({
  pricingModels = new Map(),
  pricingLoading = false,
  models,
  scopeIsEmpty,
  onClearFilters,
}: ModelAccessListProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const availability = getModelAvailabilityConfig(t)
  const [pagination, setPagination] = useState<ModelAccessPaginationState>(
    () => ({
      models,
      scopeIsEmpty,
      visibleCount: MODEL_ACCESS_PAGE_SIZE,
    })
  )

  if (models.length === 0) {
    return (
      <Empty className='min-h-64 border'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <HugeiconsIcon
              icon={PackageIcon}
              strokeWidth={2}
              aria-hidden='true'
            />
          </EmptyMedia>
          <EmptyTitle>
            {scopeIsEmpty
              ? t('No available models')
              : t('No models match the selected filters')}
          </EmptyTitle>
          {scopeIsEmpty && (
            <EmptyDescription>
              {t(
                'No models are available in the current scope. Contact your administrator to check model or channel configuration.'
              )}
            </EmptyDescription>
          )}
          {!scopeIsEmpty && (
            <EmptyDescription>
              {t('No models match your current filters.')}
            </EmptyDescription>
          )}
        </EmptyHeader>
        {!scopeIsEmpty && (
          <EmptyContent>
            <Button variant='outline' size='sm' onClick={onClearFilters}>
              {t('Clear filters')}
            </Button>
          </EmptyContent>
        )}
      </Empty>
    )
  }

  const visibleCount = getEffectiveVisibleModelCount(
    pagination,
    models,
    scopeIsEmpty
  )
  const visibleModels = models.slice(0, visibleCount)
  const hasMoreModels = visibleModels.length < models.length

  return (
    <TooltipProvider>
      <div className='flex flex-col gap-3'>
        <ItemGroup className='gap-2.5'>
          {visibleModels.map((model) => {
            const officiallyUnsupported =
              model.availability_status === 'official_unsupported'
            const availabilityConfig =
              availability[
                normalizeModelAvailabilityStatus(model.availability_status)
              ] || availability.unknown_failure
            const endpointLabels = [
              ...new Set(
                model.supported_endpoint_types.map((endpoint) =>
                  getModelEndpointLabel(endpoint, t)
                )
              ),
            ]
            const pricing = pricingModels.get(model.id)
            const requestPrice =
              pricing?.quota_type === QUOTA_TYPE_VALUES.REQUEST
                ? formatFixedPrice(pricing, '__official__', false, 1, 1, {
                    __official__: 1,
                  })
                : null
            const inputPrice =
              pricing?.quota_type === QUOTA_TYPE_VALUES.TOKEN
                ? formatGroupPrice(
                    pricing,
                    '__official__',
                    'input',
                    DEFAULT_TOKEN_UNIT,
                    false,
                    1,
                    1,
                    { __official__: 1 }
                  )
                : null
            const outputPrice =
              pricing?.quota_type === QUOTA_TYPE_VALUES.TOKEN
                ? formatGroupPrice(
                    pricing,
                    '__official__',
                    'output',
                    DEFAULT_TOKEN_UNIT,
                    false,
                    1,
                    1,
                    { __official__: 1 }
                  )
                : null
            const showPricingPanel =
              pricingLoading ||
              requestPrice !== null ||
              inputPrice !== null ||
              outputPrice !== null
            let pricingPanelContent: ReactNode
            if (pricingLoading && !pricing) {
              pricingPanelContent = (
                <div className='grid w-full grid-cols-2 gap-3'>
                  <Skeleton className='col-span-2 h-3 w-20' />
                  <Skeleton className='h-9 w-full' />
                  <Skeleton className='h-9 w-full' />
                </div>
              )
            } else if (requestPrice !== null) {
              pricingPanelContent = (
                <div className='flex min-w-0 flex-col gap-1'>
                  <span className='text-muted-foreground text-xs'>
                    {t('Per Request')}
                  </span>
                  <span className='text-base font-semibold tabular-nums'>
                    {requestPrice}
                  </span>
                </div>
              )
            } else {
              pricingPanelContent = (
                <div className='grid w-full grid-cols-2 gap-x-4 gap-y-1'>
                  <span className='text-muted-foreground col-span-2 text-xs'>
                    {t('Per 1M tokens')}
                  </span>
                  <div className='flex min-w-0 flex-col gap-0.5'>
                    <span className='text-muted-foreground text-xs'>
                      {t('Input')}
                    </span>
                    <span className='truncate text-sm font-semibold tabular-nums'>
                      {inputPrice}
                    </span>
                  </div>
                  <div className='border-border flex min-w-0 flex-col gap-0.5 border-l pl-4'>
                    <span className='text-muted-foreground text-xs'>
                      {t('Output')}
                    </span>
                    <span className='truncate text-sm font-semibold tabular-nums'>
                      {outputPrice}
                    </span>
                  </div>
                </div>
              )
            }

            return (
              <Item
                key={model.id}
                variant='outline'
                size='sm'
                className={cn(
                  '@container flex items-stretch gap-0 overflow-hidden p-0',
                  showPricingPanel && 'flex-wrap',
                  officiallyUnsupported &&
                    'border-destructive/40 bg-destructive/5'
                )}
              >
                <div
                  className={cn(
                    'flex min-w-0 flex-1 flex-col justify-center gap-2.5 px-3 py-2.5',
                    showPricingPanel && 'grow-[2] basis-[22rem]'
                  )}
                >
                  <div className='flex min-w-0 items-center gap-2.5'>
                    <ItemMedia className='translate-y-0 self-center'>
                      {getLobeIcon(model.vendor?.icon, 32)}
                    </ItemMedia>
                    <ItemContent className='min-w-0 gap-0.5'>
                      <ItemTitle className='max-w-full font-mono'>
                        <span className='truncate'>{model.id}</span>
                      </ItemTitle>
                      <ItemDescription className='text-xs leading-4'>
                        {model.vendor?.name ?? t('Unknown')}
                      </ItemDescription>
                    </ItemContent>
                    <ItemActions>
                      {officiallyUnsupported ? (
                        <Badge variant='destructive'>
                          <HugeiconsIcon
                            icon={Alert02Icon}
                            strokeWidth={2}
                            data-icon='inline-start'
                            aria-hidden='true'
                          />
                          {t('Not callable · Officially unsupported')}
                        </Badge>
                      ) : (
                        <StatusBadge
                          label={availabilityConfig.label}
                          variant={availabilityConfig.variant}
                          copyable={false}
                        />
                      )}
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <Button
                              type='button'
                              size='icon-sm'
                              variant='ghost'
                              aria-label={t('Copy to clipboard')}
                              onClick={() => void copyToClipboard(model.id)}
                            />
                          }
                        >
                          <HugeiconsIcon
                            icon={Copy01Icon}
                            strokeWidth={2}
                            aria-hidden='true'
                          />
                        </TooltipTrigger>
                        <TooltipContent>
                          {t('Copy to clipboard')}
                        </TooltipContent>
                      </Tooltip>
                    </ItemActions>
                  </div>
                  {officiallyUnsupported && (
                    <p className='text-destructive text-xs font-medium'>
                      {t(
                        'This model cannot be called because upstreams no longer support it.'
                      )}
                    </p>
                  )}
                  <ItemFooter className='justify-start pl-[42px]'>
                    <div className='flex flex-wrap gap-1.5'>
                      {endpointLabels.length > 0 ? (
                        endpointLabels.map((endpoint) => (
                          <Badge key={endpoint} variant='outline'>
                            {endpoint}
                          </Badge>
                        ))
                      ) : (
                        <Badge variant='outline'>
                          {t('Endpoint not specified')}
                        </Badge>
                      )}
                    </div>
                  </ItemFooter>
                </div>
                {showPricingPanel && (
                  <div className='bg-muted/25 border-border flex grow basis-44 items-center border-t px-3 py-2.5 @min-[33rem]:border-t-0 @min-[33rem]:border-l'>
                    {pricingPanelContent}
                  </div>
                )}
              </Item>
            )
          })}
        </ItemGroup>
        {hasMoreModels && (
          <Button
            type='button'
            variant='outline'
            className='self-center'
            onClick={() =>
              setPagination({
                models,
                scopeIsEmpty,
                visibleCount: getNextVisibleModelCount(
                  visibleCount,
                  models.length
                ),
              })
            }
          >
            {t('More')}
          </Button>
        )}
      </div>
    </TooltipProvider>
  )
}
