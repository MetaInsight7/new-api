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
import { useMemo, useState } from 'react'
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
import { useBillingTime } from '@/features/pricing/hooks/use-billing-time'
import type { PricingModel } from '@/features/pricing/types'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { toIntlLocale } from '@/i18n/languages'
import { getLobeIcon } from '@/lib/lobe-icon'
import { getModelAvailabilityConfig } from '@/lib/model-availability'
import { cn } from '@/lib/utils'

import { normalizeModelAvailabilityStatus } from '../lib/model-access-browser'
import {
  getModelAccessPricing,
  type ModelAccessPricing,
} from '../lib/model-access-pricing'
import type { ModelAccessModel } from '../types'
import { ModelAccessRatioBadge } from './model-access-ratio-badge'

type ModelAccessPricingPresentation = 'compact' | 'comparison'

type ModelAccessListProps = {
  defaultRatio: number | null
  modelRatios: Readonly<Record<string, number>>
  pricingModels?: ReadonlyMap<string, PricingModel>
  pricingLoading?: boolean
  pricingPresentation?: ModelAccessPricingPresentation
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

type ModelAccessPriceSectionProps = {
  kind: 'effective' | 'official'
  meta: string
  pricing: ModelAccessPricing | null
  referencePricing: ModelAccessPricing
  title: string
  unavailableDescription?: string
}

function ModelAccessPriceSection({
  kind,
  meta,
  pricing,
  referencePricing,
  title,
  unavailableDescription,
}: ModelAccessPriceSectionProps) {
  const { t } = useTranslation()
  const hasRequestPrice = referencePricing.requestPrice !== null
  const valueClassName = cn(
    'block truncate font-mono text-sm leading-5 font-semibold tabular-nums',
    kind === 'official' && 'text-foreground/70'
  )

  return (
    <section
      data-price-kind={kind}
      className={cn(
        'min-w-0 px-3 py-2.5',
        kind === 'effective' && 'bg-primary/[0.055] dark:bg-primary/[0.09]',
        kind === 'official' &&
          'border-border border-t @min-[35rem]:border-t-0 @min-[35rem]:border-l'
      )}
    >
      <div className='mb-1.5 flex min-w-0 items-center justify-between gap-2'>
        <span
          className={cn(
            'truncate text-[10px] font-semibold',
            kind === 'effective' ? 'text-primary' : 'text-muted-foreground'
          )}
        >
          {title}
        </span>
        <span className='text-muted-foreground shrink-0 text-[10px] font-medium'>
          {meta}
        </span>
      </div>
      {hasRequestPrice ? (
        <div className='min-w-0'>
          <span className='text-muted-foreground block text-[10px] leading-4'>
            {t('Price')}
          </span>
          <strong className={valueClassName}>
            {pricing?.requestPrice ?? '—'}
          </strong>
        </div>
      ) : (
        <div
          data-slot='comparison-primary-price-metrics'
          className='grid grid-cols-2'
        >
          <div className='min-w-0 pr-2.5'>
            <span className='text-muted-foreground block text-[10px] leading-4'>
              {t('Input')}
            </span>
            <strong className={valueClassName}>
              {pricing?.inputPrice ?? '—'}
            </strong>
          </div>
          <div className='border-border min-w-0 border-l pl-2.5'>
            <span className='text-muted-foreground block text-[10px] leading-4'>
              {t('Output')}
            </span>
            <strong className={valueClassName}>
              {pricing?.outputPrice ?? '—'}
            </strong>
          </div>
        </div>
      )}
      {referencePricing.cachePrices.length > 0 && (
        <div
          data-slot='comparison-cache-price-metrics'
          className='text-muted-foreground mt-1.5 flex max-w-full min-w-0 flex-wrap gap-x-2.5 gap-y-0.5 overflow-hidden text-[10px] leading-4'
        >
          {referencePricing.cachePrices.map((referenceCachePrice) => {
            const cachePrice = pricing?.cachePrices.find(
              (candidate) => candidate.key === referenceCachePrice.key
            )
            return (
              <span key={referenceCachePrice.key} className='min-w-0 truncate'>
                {t(referenceCachePrice.label)}{' '}
                <strong
                  className={cn(
                    'text-foreground font-mono text-[11px] font-semibold tabular-nums',
                    kind === 'official' && 'text-foreground/70'
                  )}
                >
                  {cachePrice?.price ?? '—'}
                </strong>
              </span>
            )
          })}
        </div>
      )}
      {unavailableDescription && (
        <p className='text-muted-foreground mt-1.5 truncate text-[10px] leading-4'>
          {unavailableDescription}
        </p>
      )}
    </section>
  )
}

function ModelAccessPricingPanel({
  effectiveRatio,
  presentation,
  pricing,
  pricingLoading,
}: {
  effectiveRatio: number | null
  presentation: ModelAccessPricingPresentation
  pricing?: PricingModel
  pricingLoading: boolean
}) {
  const { t } = useTranslation()
  const billingExpressions = useMemo(
    () => [
      pricing?.billing_expr,
      ...(pricing?.billing_plugin_variants?.map(
        (variant) => variant.billing_expr
      ) ?? []),
    ],
    [pricing]
  )
  const billingTime = useBillingTime(billingExpressions)
  const officialPricing = useMemo(
    () =>
      pricing
        ? getModelAccessPricing(pricing, {
            now: billingTime === undefined ? undefined : new Date(billingTime),
          })
        : null,
    [billingTime, pricing]
  )
  const effectivePricing = useMemo(
    () =>
      pricing && effectiveRatio !== null
        ? getModelAccessPricing(pricing, {
            now: billingTime === undefined ? undefined : new Date(billingTime),
            ratio: effectiveRatio,
          })
        : null,
    [billingTime, effectiveRatio, pricing]
  )

  if (pricingLoading && !pricing) {
    if (presentation === 'comparison') {
      return (
        <div className='border-border bg-muted/15 grid min-w-0 grow basis-[34rem] grid-cols-1 border-t @min-[35rem]:grid-cols-2 @min-[56rem]:border-t-0 @min-[56rem]:border-l'>
          {[0, 1].map((sectionIndex) => (
            <div
              key={sectionIndex}
              className={cn(
                'min-w-0 px-3 py-2.5',
                sectionIndex === 0 &&
                  'bg-primary/[0.055] dark:bg-primary/[0.09]',
                sectionIndex === 1 &&
                  'border-border border-t @min-[35rem]:border-t-0 @min-[35rem]:border-l'
              )}
            >
              <div className='mb-1.5 flex justify-between gap-3'>
                <Skeleton className='h-3 w-24' />
                <Skeleton className='h-3 w-14' />
              </div>
              <div className='grid grid-cols-2'>
                {[0, 1].map((metricIndex) => (
                  <div key={metricIndex} className='min-w-0 pr-2.5'>
                    <Skeleton className='mb-1 h-3 w-10' />
                    <Skeleton className='h-5 w-16 max-w-full' />
                  </div>
                ))}
              </div>
              <div className='mt-1.5 flex gap-2'>
                <Skeleton className='mb-1 h-3 w-12' />
                <Skeleton className='h-3 w-14' />
              </div>
            </div>
          ))}
        </div>
      )
    }

    return (
      <div className='bg-muted/25 border-border flex grow basis-44 items-center border-t px-3 py-2.5 @min-[33rem]:border-t-0 @min-[33rem]:border-l'>
        <div className='grid w-full grid-cols-2 gap-3'>
          <Skeleton className='col-span-2 h-3 w-20' />
          <Skeleton className='h-9 w-full' />
          <Skeleton className='h-9 w-full' />
        </div>
      </div>
    )
  }

  if (!officialPricing) return null

  const hasPrice = (modelPricing: ModelAccessPricing) =>
    modelPricing.requestPrice !== null ||
    modelPricing.inputPrice !== null ||
    modelPricing.outputPrice !== null ||
    modelPricing.cachePrices.length > 0

  if (!hasPrice(officialPricing)) return null

  const priceUnit = officialPricing.requestPrice
    ? t('Per Request')
    : t('Per 1M tokens')
  const priceUnitLabel = officialPricing.isTimePricing
    ? `${priceUnit} · ${t('Current period price')}`
    : priceUnit

  if (presentation === 'comparison') {
    return (
      <div
        data-slot='comparison-pricing-panel'
        data-pricing-presentation='comparison'
        className='border-border bg-muted/15 grid min-w-0 grow basis-[34rem] grid-cols-1 border-t @min-[35rem]:grid-cols-2 @min-[56rem]:border-t-0 @min-[56rem]:border-l'
      >
        <ModelAccessPriceSection
          kind='effective'
          meta={priceUnitLabel}
          pricing={effectivePricing}
          referencePricing={officialPricing}
          title={t('Current effective price')}
          unavailableDescription={
            effectivePricing
              ? undefined
              : t('Billed by the group selected during routing')
          }
        />
        <ModelAccessPriceSection
          kind='official'
          meta={priceUnitLabel}
          pricing={officialPricing}
          referencePricing={officialPricing}
          title={t('Official base price')}
        />
      </div>
    )
  }

  const hasRequestPrice = officialPricing.requestPrice !== null
  const hasTokenPrice =
    officialPricing.inputPrice !== null || officialPricing.outputPrice !== null

  if (!hasRequestPrice && !hasTokenPrice) return null

  let compactPricingContent
  if (!effectivePricing) {
    compactPricingContent = (
      <p className='text-muted-foreground text-xs leading-5'>
        {t('Billed by the group selected during routing')}
      </p>
    )
  } else if (hasRequestPrice) {
    compactPricingContent = (
      <span className='block truncate text-base font-semibold tabular-nums'>
        {effectivePricing.requestPrice ?? '—'}
      </span>
    )
  } else {
    const compactPrimaryPrices = [
      {
        key: 'inputPrice',
        label: t('Input'),
        price: effectivePricing.inputPrice ?? '—',
      },
      {
        key: 'outputPrice',
        label: t('Output'),
        price: effectivePricing.outputPrice ?? '—',
      },
    ]
    const compactCachePrices = effectivePricing.cachePrices.map(
      (cachePrice) => ({
        key: cachePrice.key,
        label: t(cachePrice.label),
        price: cachePrice.price,
      })
    )

    compactPricingContent = (
      <div className='w-full min-w-0'>
        <div
          data-slot='compact-primary-price-metrics'
          className='grid w-full grid-cols-2'
        >
          {compactPrimaryPrices.map((price, priceIndex) => (
            <div
              key={price.key}
              className={cn(
                'min-w-0 px-1.5 first:pl-0 last:pr-0',
                priceIndex > 0 && 'border-border border-l'
              )}
            >
              <span className='text-muted-foreground block truncate text-[10px] leading-4'>
                {price.label}
              </span>
              <span className='block truncate font-mono text-sm leading-5 font-semibold tabular-nums'>
                {price.price}
              </span>
            </div>
          ))}
        </div>
        {compactCachePrices.length > 0 && (
          <div
            data-slot='compact-cache-price-metrics'
            className='text-muted-foreground mt-1 flex max-w-full min-w-0 flex-wrap gap-x-2.5 gap-y-0.5 overflow-hidden text-[10px] leading-4'
          >
            {compactCachePrices.map((price) => (
              <span key={price.key} className='min-w-0 truncate'>
                {price.label}{' '}
                <strong className='text-foreground font-mono text-[10px] font-semibold tabular-nums'>
                  {price.price}
                </strong>
              </span>
            ))}
          </div>
        )}
      </div>
    )
  }

  return (
    <div
      data-slot='compact-pricing-panel'
      data-pricing-presentation='compact'
      data-price-kind='effective'
      className='bg-primary/[0.055] border-border dark:bg-primary/[0.09] flex min-w-0 grow basis-full items-start border-t px-3 py-2.5 @min-[33rem]:w-[13.75rem] @min-[33rem]:shrink-0 @min-[33rem]:grow-0 @min-[33rem]:basis-[13.75rem] @min-[33rem]:border-t-0 @min-[33rem]:border-l @min-[33rem]:pl-[11px]'
    >
      <div className='w-full min-w-0'>
        <div className='mb-1.5 flex min-w-0 items-center justify-between gap-2'>
          <span className='text-primary truncate text-[10px] font-semibold'>
            {t('Current effective price')}
          </span>
          <span className='text-muted-foreground shrink-0 text-[10px] font-medium'>
            {priceUnitLabel}
          </span>
        </div>
        {compactPricingContent}
      </div>
    </div>
  )
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
  defaultRatio,
  modelRatios,
  pricingModels = new Map(),
  pricingLoading = false,
  pricingPresentation = 'compact',
  models,
  scopeIsEmpty,
  onClearFilters,
}: ModelAccessListProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
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
            const hasExclusiveRatio = Object.hasOwn(modelRatios, model.id)
            const exclusiveRatio = modelRatios[model.id]
            const effectiveRatio = hasExclusiveRatio
              ? (exclusiveRatio ?? null)
              : defaultRatio
            const pricing = pricingModels.get(model.id)
            const showPricingPanel = pricingLoading || pricing !== undefined
            const exclusiveRatioText =
              exclusiveRatio === undefined
                ? ''
                : exclusiveRatio.toLocaleString(locale, {
                    maximumFractionDigits: 6,
                  })
            const defaultRatioText =
              defaultRatio === null
                ? ''
                : defaultRatio.toLocaleString(locale, {
                    maximumFractionDigits: 6,
                  })
            const exclusiveRatioLabel = hasExclusiveRatio
              ? t('Exclusive ratio {{ratio}}×', {
                  ratio: exclusiveRatioText,
                })
              : ''
            let exclusiveRatioDescription = ''
            if (hasExclusiveRatio && officiallyUnsupported) {
              exclusiveRatioDescription =
                defaultRatio === null
                  ? t(
                      'If this model becomes callable again, it will use the exclusive ratio {{ratio}}×, overriding the default ratio; the two are not multiplied.',
                      { ratio: exclusiveRatioText }
                    )
                  : t(
                      'If this model becomes callable again, it will use the exclusive ratio {{ratio}}×, overriding the default ratio {{defaultRatio}}×; the two are not multiplied.',
                      {
                        ratio: exclusiveRatioText,
                        defaultRatio: defaultRatioText,
                      }
                    )
            } else if (hasExclusiveRatio) {
              exclusiveRatioDescription =
                defaultRatio === null
                  ? t(
                      'This exclusive ratio overrides the default ratio; the two are not multiplied.'
                    )
                  : t(
                      'This exclusive ratio overrides the default ratio {{defaultRatio}}×; the two are not multiplied.',
                      { defaultRatio: defaultRatioText }
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
                    showPricingPanel &&
                      (pricingPresentation === 'comparison'
                        ? 'grow-[2] basis-[22rem]'
                        : 'grow-[2] basis-[20rem]')
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
                      {hasExclusiveRatio && (
                        <Tooltip>
                          <TooltipTrigger
                            render={
                              <ModelAccessRatioBadge
                                ratio={exclusiveRatio}
                                label={exclusiveRatioLabel}
                                tone='exclusive'
                                tabIndex={0}
                                aria-label={`${exclusiveRatioLabel}. ${exclusiveRatioDescription}`}
                              />
                            }
                          />
                          <TooltipContent>
                            {exclusiveRatioDescription}
                          </TooltipContent>
                        </Tooltip>
                      )}
                      {!hasExclusiveRatio && (
                        <ModelAccessRatioBadge
                          ratio={effectiveRatio}
                          label={
                            effectiveRatio === null
                              ? t('Auto')
                              : `${t('Group Ratio')} ${effectiveRatio.toLocaleString(locale, { maximumFractionDigits: 6 })}×`
                          }
                        />
                      )}
                    </div>
                  </ItemFooter>
                </div>
                <ModelAccessPricingPanel
                  effectiveRatio={effectiveRatio}
                  presentation={pricingPresentation}
                  pricing={pricing}
                  pricingLoading={pricingLoading}
                />
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
